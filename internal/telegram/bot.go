package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"comfy-tg-bot/internal/comfyui"
	"comfy-tg-bot/internal/config"
	"comfy-tg-bot/internal/image"
	"comfy-tg-bot/internal/limiter"
)

// Bot represents the Telegram bot
type Bot struct {
	api     *tgbotapi.BotAPI
	handler *Handler
	cfg     config.TelegramConfig
	logger  *slog.Logger

	// Track active message processing
	activeRequests sync.WaitGroup
}

// NewBot creates a new Telegram bot
func NewBot(
	cfg config.TelegramConfig,
	comfyClient *comfyui.Client,
	imageProcessor *image.Processor,
	userLimiter *limiter.UserLimiter,
	logger *slog.Logger,
) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}

	whitelist := NewWhitelist(cfg.AllowedUsers, logger)
	handler := NewHandler(api, comfyClient, imageProcessor, whitelist, userLimiter, logger)

	return &Bot{
		api:     api,
		handler: handler,
		cfg:     cfg,
		logger:  logger,
	}, nil
}

// Run starts the bot and blocks until context is cancelled
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.cfg.PollingTimeout

	updates := b.api.GetUpdatesChan(u)

	b.logger.Info("bot started", "username", b.api.Self.UserName)

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("stopping bot, waiting for active requests")

			// Stop receiving updates
			b.api.StopReceivingUpdates()

			// Wait for active requests with timeout
			done := make(chan struct{})
			go func() {
				b.activeRequests.Wait()
				close(done)
			}()

			select {
			case <-done:
				b.logger.Info("all active requests completed")
			case <-time.After(25 * time.Second):
				b.logger.Warn("some requests may not have completed")
			}

			return ctx.Err()

		case update, ok := <-updates:
			if !ok {
				return nil
			}

			// Process update in goroutine
			b.activeRequests.Add(1)
			go func(upd tgbotapi.Update) {
				defer b.activeRequests.Done()

				// Create request context with timeout
				reqCtx, cancel := context.WithTimeout(ctx, b.cfg.RequestTimeout)
				defer cancel()

				b.handler.HandleUpdate(reqCtx, upd)
			}(update)
		}
	}
}

// GetBotInfo returns information about the bot
func (b *Bot) GetBotInfo() tgbotapi.User {
	return b.api.Self
}
