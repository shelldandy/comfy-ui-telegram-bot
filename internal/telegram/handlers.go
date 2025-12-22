package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"comfy-tg-bot/internal/comfyui"
	apperrors "comfy-tg-bot/internal/errors"
	"comfy-tg-bot/internal/image"
	"comfy-tg-bot/internal/limiter"
)

// Handler processes Telegram updates
type Handler struct {
	bot       *tgbotapi.BotAPI
	comfy     *comfyui.Client
	processor *image.Processor
	whitelist *Whitelist
	limiter   *limiter.UserLimiter
	logger    *slog.Logger
}

// NewHandler creates a new update handler
func NewHandler(
	bot *tgbotapi.BotAPI,
	comfy *comfyui.Client,
	processor *image.Processor,
	whitelist *Whitelist,
	limiter *limiter.UserLimiter,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		bot:       bot,
		comfy:     comfy,
		processor: processor,
		whitelist: whitelist,
		limiter:   limiter,
		logger:    logger,
	}
}

// HandleUpdate processes a single update
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Check whitelist
	userID, allowed := h.whitelist.CheckAccess(update)
	if !allowed {
		if update.Message != nil {
			h.sendText(update.Message.Chat.ID, apperrors.ErrUnauthorized.UserMsg)
		}
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message

	// Handle commands
	if msg.IsCommand() {
		h.handleCommand(ctx, msg)
		return
	}

	// Handle text messages as prompts
	if msg.Text != "" {
		h.handlePrompt(ctx, msg, userID)
	}
}

func (h *Handler) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		h.sendText(msg.Chat.ID,
			"Welcome to the ComfyUI Bot!\n\n"+
				"Send me a text prompt and I'll generate an image for you.\n\n"+
				"Commands:\n"+
				"/help - Show this help message\n"+
				"/status - Check ComfyUI server status")

	case "help":
		h.sendText(msg.Chat.ID,
			"Simply send me a text description of the image you want to generate.\n\n"+
				"For example: \"A beautiful sunset over mountains with a lake reflection\"\n\n"+
				"You'll receive both the original PNG and a compressed JPEG version.")

	case "status":
		h.handleStatus(ctx, msg)

	default:
		h.sendText(msg.Chat.ID, "Unknown command. Use /help for available commands.")
	}
}

func (h *Handler) handleStatus(ctx context.Context, msg *tgbotapi.Message) {
	err := h.comfy.CheckHealth(ctx)
	if err != nil {
		h.sendText(msg.Chat.ID, fmt.Sprintf("ComfyUI Status: Offline\nError: %v", err))
		return
	}

	activeCount := h.limiter.ActiveCount()
	h.sendText(msg.Chat.ID, fmt.Sprintf(
		"ComfyUI Status: Online\n"+
			"Active generations: %d", activeCount))
}

func (h *Handler) handlePrompt(ctx context.Context, msg *tgbotapi.Message, userID int64) {
	prompt := strings.TrimSpace(msg.Text)

	if len(prompt) < 3 {
		h.sendText(msg.Chat.ID, "Please provide a more detailed prompt (at least 3 characters).")
		return
	}

	// Check if user already has an active request
	if !h.limiter.TryAcquire(userID) {
		h.sendText(msg.Chat.ID, apperrors.ErrGenerationInProgress.UserMsg)
		return
	}
	defer h.limiter.Release(userID)

	// Send "generating" message
	statusMsg, err := h.bot.Send(tgbotapi.NewMessage(msg.Chat.ID, "Generating your image..."))
	if err != nil {
		h.logger.Error("failed to send status message", "error", err)
	}

	// Generate image
	h.logger.Info("starting generation", "user_id", userID, "prompt_length", len(prompt))

	imageData, err := h.comfy.GenerateImage(ctx, prompt)
	if err != nil {
		h.logger.Error("generation failed", "error", err, "user_id", userID)
		h.sendText(msg.Chat.ID, apperrors.GetUserMessage(err))

		// Delete status message on error
		if statusMsg.MessageID != 0 {
			h.bot.Request(tgbotapi.NewDeleteMessage(msg.Chat.ID, statusMsg.MessageID))
		}
		return
	}

	// Process image
	result, err := h.processor.Process(imageData)
	if err != nil {
		h.logger.Error("image processing failed", "error", err)
		h.sendText(msg.Chat.ID, "Failed to process the generated image.")
		return
	}

	h.logger.Info("generation complete",
		"user_id", userID,
		"original_size", result.OriginalSize,
		"compressed_size", result.CompressedSize,
	)

	// Delete "generating" message
	if statusMsg.MessageID != 0 {
		h.bot.Request(tgbotapi.NewDeleteMessage(msg.Chat.ID, statusMsg.MessageID))
	}

	// Send compressed version as photo (for preview)
	photoMsg := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  "image.jpg",
		Bytes: result.Compressed,
	})
	photoMsg.Caption = fmt.Sprintf("Prompt: %s", truncate(prompt, 200))
	if _, err := h.bot.Send(photoMsg); err != nil {
		h.logger.Error("failed to send photo", "error", err)
	}

	// Send original as document
	docMsg := tgbotapi.NewDocument(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  "image.png",
		Bytes: result.Original,
	})
	docMsg.Caption = "Original PNG"
	if _, err := h.bot.Send(docMsg); err != nil {
		h.logger.Error("failed to send document", "error", err)
	}
}

func (h *Handler) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := h.bot.Send(msg); err != nil {
		h.logger.Error("failed to send message", "error", err, "chat_id", chatID)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
