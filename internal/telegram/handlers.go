package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"

	"comfy-tg-bot/internal/admin"
	"comfy-tg-bot/internal/comfyui"
	apperrors "comfy-tg-bot/internal/errors"
	"comfy-tg-bot/internal/generations"
	"comfy-tg-bot/internal/image"
	"comfy-tg-bot/internal/limiter"
)

// Handler processes Telegram updates
type Handler struct {
	bot         *tgbotapi.BotAPI
	comfy       *comfyui.Client
	processor   *image.Processor
	whitelist   *Whitelist
	limiter     *limiter.UserLimiter
	generations generations.Store
	adminStore  admin.Store
	logger      *slog.Logger
}

// NewHandler creates a new update handler
func NewHandler(
	bot *tgbotapi.BotAPI,
	comfy *comfyui.Client,
	processor *image.Processor,
	whitelist *Whitelist,
	limiter *limiter.UserLimiter,
	generationsStore generations.Store,
	adminStore admin.Store,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		bot:         bot,
		comfy:       comfy,
		processor:   processor,
		whitelist:   whitelist,
		limiter:     limiter,
		generations: generationsStore,
		adminStore:  adminStore,
		logger:      logger,
	}
}

// HandleUpdate processes a single update
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Handle admin callbacks first (admin must be able to approve even if callback is from unauthorized chat)
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		if strings.HasPrefix(data, "admin:") {
			h.handleAdminCallback(ctx, update.CallbackQuery)
			return
		}
		if strings.HasPrefix(data, "admin_group:") {
			h.handleAdminGroupCallback(ctx, update.CallbackQuery)
			return
		}
	}

	// Check whitelist with group awareness
	userID, chatID, isGroup, allowed := h.whitelist.CheckAccess(update)

	if !allowed {
		if update.Message != nil {
			if isGroup {
				h.handleUnauthorizedGroup(ctx, update.Message)
			} else {
				h.handleUnauthorizedUser(ctx, update.Message)
			}
		}
		return
	}

	// Handle callback queries (inline button presses)
	if update.CallbackQuery != nil {
		if strings.HasPrefix(update.CallbackQuery.Data, "gen:") {
			h.handleGenerationCallback(ctx, update.CallbackQuery)
		}
		return
	}

	if update.Message == nil {
		return
	}

	msg := update.Message

	// For group chats, only respond to bot mentions
	if isGroup {
		prompt, hasMention := h.parseBotMention(msg)
		if hasMention && prompt != "" {
			h.handleGroupPrompt(ctx, msg, userID, chatID, prompt)
		}
		// Ignore non-mention messages in groups
		return
	}

	// Handle commands (private chats only)
	if msg.IsCommand() {
		h.handleCommand(ctx, msg)
		return
	}

	// Handle text messages as prompts (private chats)
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
		helpText := "Simply send me a text description of the image you want to generate.\n\n" +
			"For example: \"A beautiful sunset over mountains with a lake reflection\"\n\n" +
			"In groups, mention me with @" + h.bot.Self.UserName + " followed by your prompt.\n\n" +
			"Each generated image comes with buttons to copy the original prompt or " +
			"download it in full resolution.\n\n" +
			"Commands:\n" +
			"/status - Check ComfyUI server status"

		if h.whitelist.IsAdmin(msg.From.ID) {
			helpText += "\n\nAdmin commands:\n" +
				"/revoke <user_id> - Revoke user access\n" +
				"/revokegroup <group_id> - Revoke group access"
		}

		h.sendText(msg.Chat.ID, helpText)

	case "status":
		h.handleStatus(ctx, msg)

	case "revoke":
		h.handleRevoke(ctx, msg)

	case "revokegroup":
		h.handleRevokeGroup(ctx, msg)

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

	imageData, imgRef, err := h.comfy.GenerateImage(ctx, prompt)
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

	// Persist the prompt + ComfyUI file reference so the action buttons can
	// recall the full prompt and re-download the original on demand.
	genID := h.saveGeneration(userID, prompt, imgRef)

	// Send compressed version as photo (for preview) with action buttons
	photoMsg := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  "image.jpg",
		Bytes: result.Compressed,
	})
	photoMsg.Caption = fmt.Sprintf("Prompt: %s", truncate(prompt, 200))
	if genID != "" {
		photoMsg.ReplyMarkup = generationKeyboard(genID)
	}
	if _, err := h.bot.Send(photoMsg); err != nil {
		h.logger.Error("failed to send photo", "error", err)
	}
}

// saveGeneration persists a generation record and returns its short ID, used as
// the key in the action buttons' callback data. Returns "" on failure (the image
// is still sent, just without working buttons).
func (h *Handler) saveGeneration(userID int64, prompt string, ref comfyui.ImageOutput) string {
	id := uuid.NewString()[:12]
	rec := &generations.Record{
		ID:        id,
		UserID:    userID,
		Prompt:    prompt,
		Filename:  ref.Filename,
		Subfolder: ref.Subfolder,
		ImgType:   ref.Type,
		CreatedAt: time.Now(),
	}
	if err := h.generations.Save(rec); err != nil {
		h.logger.Error("failed to save generation", "error", err, "user_id", userID)
		return ""
	}
	return id
}

// generationKeyboard builds the two action buttons attached to a generated image.
func generationKeyboard(id string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Copy prompt", "gen:prompt:"+id),
			tgbotapi.NewInlineKeyboardButtonData("🖼 Original size", "gen:orig:"+id),
		),
	)
}

// handleGenerationCallback services the "Copy prompt" and "Original size" buttons.
func (h *Handler) handleGenerationCallback(ctx context.Context, query *tgbotapi.CallbackQuery) {
	parts := strings.SplitN(strings.TrimPrefix(query.Data, "gen:"), ":", 2)
	if len(parts) != 2 {
		h.answerCallback(query.ID, "Invalid action")
		return
	}
	action, id := parts[0], parts[1]

	rec, err := h.generations.Get(id)
	if err != nil {
		h.logger.Error("failed to get generation", "error", err, "id", id)
		h.answerCallback(query.ID, "Something went wrong")
		return
	}
	if rec == nil {
		h.answerCallback(query.ID, "This image is no longer available.")
		return
	}

	chatID := query.Message.Chat.ID

	switch action {
	case "prompt":
		// Send the full prompt as a tap-to-copy code block.
		reply := tgbotapi.NewMessage(chatID, "<code>"+tgbotapi.EscapeText(tgbotapi.ModeHTML, rec.Prompt)+"</code>")
		reply.ParseMode = tgbotapi.ModeHTML
		reply.ReplyToMessageID = query.Message.MessageID
		if _, err := h.bot.Send(reply); err != nil {
			h.logger.Error("failed to send prompt", "error", err)
		}
		h.answerCallback(query.ID, "Prompt sent")

	case "orig":
		h.answerCallback(query.ID, "Fetching original…")
		data, err := h.comfy.GetImage(ctx, rec.Filename, rec.Subfolder, rec.ImgType)
		if err != nil {
			h.logger.Error("failed to fetch original image", "error", err, "id", id)
			h.sendText(chatID, "Couldn't retrieve the original image.")
			return
		}
		docMsg := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
			Name:  "image.png",
			Bytes: data,
		})
		docMsg.ReplyToMessageID = query.Message.MessageID
		if _, err := h.bot.Send(docMsg); err != nil {
			h.logger.Error("failed to send original document", "error", err)
		}

	default:
		h.answerCallback(query.ID, "Unknown action")
	}
}

func (h *Handler) answerCallback(callbackID string, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := h.bot.Request(callback); err != nil {
		h.logger.Error("failed to answer callback", "error", err)
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

// handleUnauthorizedUser handles access attempts from non-whitelisted users
func (h *Handler) handleUnauthorizedUser(ctx context.Context, msg *tgbotapi.Message) {
	// If no admin is configured, just send the unauthorized message
	if h.whitelist.AdminUserID() == 0 || h.adminStore == nil {
		h.sendText(msg.Chat.ID, apperrors.ErrUnauthorized.UserMsg)
		return
	}

	userID := msg.From.ID

	// Check if already pending
	pending, err := h.adminStore.GetPending(userID)
	if err != nil {
		h.logger.Error("failed to check pending status", "error", err, "user_id", userID)
		h.sendText(msg.Chat.ID, apperrors.ErrUnauthorized.UserMsg)
		return
	}

	if pending != nil && pending.NotifiedAt != nil {
		// Already notified admin, just inform user
		h.sendText(msg.Chat.ID, "Your access request is pending admin approval.")
		return
	}

	// Add to pending if not exists
	if pending == nil {
		req := admin.PendingRequest{
			UserID:      userID,
			Username:    msg.From.UserName,
			FirstName:   msg.From.FirstName,
			ChatID:      msg.Chat.ID,
			RequestedAt: time.Now(),
		}
		if err := h.adminStore.AddPending(req); err != nil {
			h.logger.Error("failed to add pending request", "error", err, "user_id", userID)
			h.sendText(msg.Chat.ID, apperrors.ErrUnauthorized.UserMsg)
			return
		}
	}

	// Notify admin
	adminMsgID := h.notifyAdmin(userID, msg.From.UserName, msg.From.FirstName)
	if adminMsgID > 0 {
		if err := h.adminStore.UpdatePendingNotified(userID, adminMsgID); err != nil {
			h.logger.Error("failed to update pending notified", "error", err, "user_id", userID)
		}
	}

	h.sendText(msg.Chat.ID, "Your access request has been sent to the admin for approval.")
}

// notifyAdmin sends an approval request to the admin
func (h *Handler) notifyAdmin(userID int64, username, firstName string) int {
	adminChatID := h.whitelist.AdminUserID()

	usernameDisplay := username
	if usernameDisplay == "" {
		usernameDisplay = "(none)"
	} else {
		usernameDisplay = "@" + usernameDisplay
	}

	nameDisplay := firstName
	if nameDisplay == "" {
		nameDisplay = "(none)"
	}

	text := fmt.Sprintf(
		"New access request:\n\n"+
			"User ID: %d\n"+
			"Username: %s\n"+
			"Name: %s",
		userID, usernameDisplay, nameDisplay,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("admin:approve:%d", userID)),
			tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("admin:reject:%d", userID)),
		),
	)

	msg := tgbotapi.NewMessage(adminChatID, text)
	msg.ReplyMarkup = keyboard

	sent, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("failed to notify admin", "error", err)
		return 0
	}
	return sent.MessageID
}

// handleAdminCallback handles approve/reject callbacks from the admin
func (h *Handler) handleAdminCallback(ctx context.Context, query *tgbotapi.CallbackQuery) {
	if !h.whitelist.IsAdmin(query.From.ID) {
		h.answerCallback(query.ID, "Unauthorized")
		return
	}

	data := query.Data
	parts := strings.Split(strings.TrimPrefix(data, "admin:"), ":")
	if len(parts) != 2 {
		h.answerCallback(query.ID, "Invalid action")
		return
	}

	action := parts[0]
	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		h.answerCallback(query.ID, "Invalid user ID")
		return
	}

	pending, err := h.adminStore.GetPending(userID)
	if err != nil {
		h.logger.Error("failed to get pending request", "error", err, "user_id", userID)
		h.answerCallback(query.ID, "Failed to get request")
		return
	}

	if pending == nil {
		h.answerCallback(query.ID, "Request not found or already processed")
		return
	}

	switch action {
	case "approve":
		approved := admin.ApprovedUser{
			UserID:     userID,
			Username:   pending.Username,
			ApprovedAt: time.Now(),
			ApprovedBy: query.From.ID,
		}
		if err := h.adminStore.AddApproved(approved); err != nil {
			h.logger.Error("failed to approve user", "error", err, "user_id", userID)
			h.answerCallback(query.ID, "Failed to approve")
			return
		}
		if err := h.adminStore.RemovePending(userID); err != nil {
			h.logger.Error("failed to remove pending", "error", err, "user_id", userID)
		}

		// Notify user they were approved
		h.sendText(pending.ChatID, "Your access has been approved! You can now use the bot.")

		// Update admin message
		usernameDisplay := pending.Username
		if usernameDisplay == "" {
			usernameDisplay = "(none)"
		} else {
			usernameDisplay = "@" + usernameDisplay
		}
		h.updateAdminMessage(query.Message.Chat.ID, query.Message.MessageID,
			fmt.Sprintf("User %d (%s) approved", userID, usernameDisplay))

		h.answerCallback(query.ID, "User approved")

	case "reject":
		if err := h.adminStore.RemovePending(userID); err != nil {
			h.logger.Error("failed to remove pending", "error", err, "user_id", userID)
		}

		// Notify user they were rejected
		h.sendText(pending.ChatID, "Your access request was denied.")

		// Update admin message
		usernameDisplay := pending.Username
		if usernameDisplay == "" {
			usernameDisplay = "(none)"
		} else {
			usernameDisplay = "@" + usernameDisplay
		}
		h.updateAdminMessage(query.Message.Chat.ID, query.Message.MessageID,
			fmt.Sprintf("User %d (%s) rejected", userID, usernameDisplay))

		h.answerCallback(query.ID, "User rejected")

	default:
		h.answerCallback(query.ID, "Unknown action")
	}
}

// updateAdminMessage updates an admin notification message
func (h *Handler) updateAdminMessage(chatID int64, msgID int, newText string) {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, newText)
	if _, err := h.bot.Send(edit); err != nil {
		h.logger.Error("failed to update admin message", "error", err)
	}
}

// handleRevoke handles the /revoke command for admins
func (h *Handler) handleRevoke(ctx context.Context, msg *tgbotapi.Message) {
	if !h.whitelist.IsAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "This command is only available to admins.")
		return
	}

	if h.adminStore == nil {
		h.sendText(msg.Chat.ID, "Admin features are not configured.")
		return
	}

	args := msg.CommandArguments()
	if args == "" {
		h.sendText(msg.Chat.ID, "Usage: /revoke <user_id>")
		return
	}

	userID, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
	if err != nil {
		h.sendText(msg.Chat.ID, "Invalid user ID. Usage: /revoke <user_id>")
		return
	}

	if err := h.adminStore.RemoveApproved(userID); err != nil {
		h.logger.Error("failed to revoke user", "error", err, "user_id", userID)
		h.sendText(msg.Chat.ID, "Failed to revoke user access.")
		return
	}

	h.sendText(msg.Chat.ID, fmt.Sprintf("User %d access has been revoked.", userID))
}

// parseBotMention checks if the message contains a mention of the bot
// and extracts the prompt text after/around the mention
func (h *Handler) parseBotMention(msg *tgbotapi.Message) (string, bool) {
	if msg.Text == "" {
		return "", false
	}

	botUsername := "@" + h.bot.Self.UserName

	// Check if message contains bot mention (case-insensitive)
	if !strings.Contains(strings.ToLower(msg.Text), strings.ToLower(botUsername)) {
		return "", false
	}

	// Check entities for proper mention detection
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			mentionText := msg.Text[entity.Offset : entity.Offset+entity.Length]
			if strings.EqualFold(mentionText, botUsername) {
				// Extract text before and after the mention
				beforeMention := strings.TrimSpace(msg.Text[:entity.Offset])
				afterMention := strings.TrimSpace(msg.Text[entity.Offset+entity.Length:])

				// Combine both parts as prompt
				var prompt string
				if beforeMention != "" && afterMention != "" {
					prompt = beforeMention + " " + afterMention
				} else if beforeMention != "" {
					prompt = beforeMention
				} else {
					prompt = afterMention
				}

				return prompt, true
			}
		}
	}

	// Fallback: case-insensitive replacement if entities don't match
	lowerText := strings.ToLower(msg.Text)
	lowerUsername := strings.ToLower(botUsername)
	idx := strings.Index(lowerText, lowerUsername)
	if idx >= 0 {
		prompt := strings.TrimSpace(msg.Text[:idx] + msg.Text[idx+len(botUsername):])
		return prompt, true
	}

	return "", false
}

// handleGroupPrompt handles image generation requests from groups
func (h *Handler) handleGroupPrompt(ctx context.Context, msg *tgbotapi.Message, userID, groupID int64, prompt string) {
	prompt = strings.TrimSpace(prompt)

	if len(prompt) < 3 {
		h.sendText(msg.Chat.ID, "Please provide a more detailed prompt (at least 3 characters).")
		return
	}

	// Check if user already has an active request (rate limit per user, not per group)
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
	h.logger.Info("starting group generation",
		"user_id", userID,
		"group_id", groupID,
		"prompt_length", len(prompt))

	imageData, imgRef, err := h.comfy.GenerateImage(ctx, prompt)
	if err != nil {
		h.logger.Error("generation failed", "error", err, "user_id", userID, "group_id", groupID)
		h.sendText(msg.Chat.ID, apperrors.GetUserMessage(err))

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

	h.logger.Info("group generation complete",
		"user_id", userID,
		"group_id", groupID,
		"compressed_size", result.CompressedSize,
	)

	// Delete "generating" message
	if statusMsg.MessageID != 0 {
		h.bot.Request(tgbotapi.NewDeleteMessage(msg.Chat.ID, statusMsg.MessageID))
	}

	// Persist the prompt + ComfyUI file reference for the action buttons.
	genID := h.saveGeneration(userID, prompt, imgRef)

	// Send ONLY compressed version for groups, with action buttons
	photoMsg := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileBytes{
		Name:  "image.jpg",
		Bytes: result.Compressed,
	})
	photoMsg.Caption = fmt.Sprintf("Prompt: %s", truncate(prompt, 200))
	photoMsg.ReplyToMessageID = msg.MessageID // Reply to the original request
	if genID != "" {
		photoMsg.ReplyMarkup = generationKeyboard(genID)
	}

	if _, err := h.bot.Send(photoMsg); err != nil {
		h.logger.Error("failed to send photo to group", "error", err)
	}
}

// handleUnauthorizedGroup handles access attempts from unapproved groups
func (h *Handler) handleUnauthorizedGroup(ctx context.Context, msg *tgbotapi.Message) {
	// Only process if this is a mention of the bot
	_, hasMention := h.parseBotMention(msg)
	if !hasMention {
		return
	}

	// If no admin is configured, just ignore
	if h.whitelist.AdminUserID() == 0 || h.adminStore == nil {
		return
	}

	groupID := msg.Chat.ID
	groupTitle := msg.Chat.Title

	// Check if already pending
	pending, err := h.adminStore.GetPendingGroup(groupID)
	if err != nil {
		h.logger.Error("failed to check pending group status", "error", err, "group_id", groupID)
		return
	}

	if pending != nil && pending.NotifiedAt != nil {
		// Already notified admin, ignore further requests
		return
	}

	// Add to pending if not exists
	if pending == nil {
		req := admin.PendingGroupRequest{
			GroupID:     groupID,
			Title:       groupTitle,
			RequestedAt: time.Now(),
		}
		if err := h.adminStore.AddPendingGroup(req); err != nil {
			h.logger.Error("failed to add pending group request", "error", err, "group_id", groupID)
			return
		}
	}

	// Notify admin
	adminMsgID := h.notifyAdminAboutGroup(groupID, groupTitle)
	if adminMsgID > 0 {
		if err := h.adminStore.UpdatePendingGroupNotified(groupID, adminMsgID); err != nil {
			h.logger.Error("failed to update pending group notified", "error", err, "group_id", groupID)
		}
	}
}

// notifyAdminAboutGroup sends an approval request to the admin for a group
func (h *Handler) notifyAdminAboutGroup(groupID int64, title string) int {
	adminChatID := h.whitelist.AdminUserID()

	titleDisplay := title
	if titleDisplay == "" {
		titleDisplay = "(unnamed group)"
	}

	text := fmt.Sprintf(
		"New group access request:\n\n"+
			"Group ID: %d\n"+
			"Title: %s",
		groupID, titleDisplay,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("admin_group:approve:%d", groupID)),
			tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("admin_group:reject:%d", groupID)),
		),
	)

	msg := tgbotapi.NewMessage(adminChatID, text)
	msg.ReplyMarkup = keyboard

	sent, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("failed to notify admin about group", "error", err)
		return 0
	}
	return sent.MessageID
}

// handleAdminGroupCallback handles approve/reject callbacks for groups
func (h *Handler) handleAdminGroupCallback(ctx context.Context, query *tgbotapi.CallbackQuery) {
	if !h.whitelist.IsAdmin(query.From.ID) {
		h.answerCallback(query.ID, "Unauthorized")
		return
	}

	data := query.Data
	parts := strings.Split(strings.TrimPrefix(data, "admin_group:"), ":")
	if len(parts) != 2 {
		h.answerCallback(query.ID, "Invalid action")
		return
	}

	action := parts[0]
	groupID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		h.answerCallback(query.ID, "Invalid group ID")
		return
	}

	pending, err := h.adminStore.GetPendingGroup(groupID)
	if err != nil {
		h.logger.Error("failed to get pending group request", "error", err, "group_id", groupID)
		h.answerCallback(query.ID, "Failed to get request")
		return
	}

	if pending == nil {
		h.answerCallback(query.ID, "Request not found or already processed")
		return
	}

	switch action {
	case "approve":
		approved := admin.ApprovedGroup{
			GroupID:    groupID,
			Title:      pending.Title,
			ApprovedAt: time.Now(),
			ApprovedBy: query.From.ID,
		}
		if err := h.adminStore.AddApprovedGroup(approved); err != nil {
			h.logger.Error("failed to approve group", "error", err, "group_id", groupID)
			h.answerCallback(query.ID, "Failed to approve")
			return
		}
		if err := h.adminStore.RemovePendingGroup(groupID); err != nil {
			h.logger.Error("failed to remove pending group", "error", err, "group_id", groupID)
		}

		// Notify group they were approved
		h.sendText(groupID, "This group has been approved! You can now use the bot by mentioning @"+h.bot.Self.UserName+" followed by your prompt.")

		// Update admin message
		titleDisplay := pending.Title
		if titleDisplay == "" {
			titleDisplay = "(unnamed)"
		}
		h.updateAdminMessage(query.Message.Chat.ID, query.Message.MessageID,
			fmt.Sprintf("Group %d (%s) approved", groupID, titleDisplay))

		h.answerCallback(query.ID, "Group approved")

	case "reject":
		if err := h.adminStore.RemovePendingGroup(groupID); err != nil {
			h.logger.Error("failed to remove pending group", "error", err, "group_id", groupID)
		}

		// Update admin message
		titleDisplay := pending.Title
		if titleDisplay == "" {
			titleDisplay = "(unnamed)"
		}
		h.updateAdminMessage(query.Message.Chat.ID, query.Message.MessageID,
			fmt.Sprintf("Group %d (%s) rejected", groupID, titleDisplay))

		h.answerCallback(query.ID, "Group rejected")

	default:
		h.answerCallback(query.ID, "Unknown action")
	}
}

// handleRevokeGroup handles the /revokegroup command for admins
func (h *Handler) handleRevokeGroup(ctx context.Context, msg *tgbotapi.Message) {
	if !h.whitelist.IsAdmin(msg.From.ID) {
		h.sendText(msg.Chat.ID, "This command is only available to admins.")
		return
	}

	if h.adminStore == nil {
		h.sendText(msg.Chat.ID, "Admin features are not configured.")
		return
	}

	args := msg.CommandArguments()
	if args == "" {
		h.sendText(msg.Chat.ID, "Usage: /revokegroup <group_id>")
		return
	}

	groupID, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
	if err != nil {
		h.sendText(msg.Chat.ID, "Invalid group ID. Usage: /revokegroup <group_id>")
		return
	}

	if err := h.adminStore.RemoveApprovedGroup(groupID); err != nil {
		h.logger.Error("failed to revoke group", "error", err, "group_id", groupID)
		h.sendText(msg.Chat.ID, "Failed to revoke group access.")
		return
	}

	h.sendText(msg.Chat.ID, fmt.Sprintf("Group %d access has been revoked.", groupID))
}
