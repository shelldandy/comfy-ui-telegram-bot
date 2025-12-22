package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"comfy-tg-bot/internal/comfyui"
	"comfy-tg-bot/internal/config"
	"comfy-tg-bot/internal/image"
	"comfy-tg-bot/internal/limiter"
	"comfy-tg-bot/internal/telegram"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize logger
	var logLevel slog.Level
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if cfg.Logging.JSONFormat {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Create root context with cancellation
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// WaitGroup for tracking active goroutines
	var wg sync.WaitGroup

	// Initialize ComfyUI client
	comfyClient, err := comfyui.NewClient(cfg.ComfyUI, logger)
	if err != nil {
		logger.Error("failed to create comfyui client", "error", err)
		os.Exit(1)
	}

	// Initialize image processor
	imageProcessor := image.NewProcessor(cfg.Image.JPEGQuality)

	// Initialize user limiter (0 = no global limit, just per-user)
	userLimiter := limiter.NewUserLimiter(0)

	// Initialize Telegram bot
	bot, err := telegram.NewBot(cfg.Telegram, comfyClient, imageProcessor, userLimiter, logger)
	if err != nil {
		logger.Error("failed to create telegram bot", "error", err)
		os.Exit(1)
	}

	// Start bot in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := bot.Run(rootCtx); err != nil && err != context.Canceled {
			logger.Error("bot error", "error", err)
		}
	}()

	logger.Info("bot started",
		"allowed_users", cfg.Telegram.AllowedUsers,
		"comfyui_url", cfg.ComfyUI.BaseURL,
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("shutdown signal received", "signal", sig)

	// Cancel root context to signal all goroutines
	rootCancel()

	// Wait for graceful shutdown with timeout
	shutdownTimeout := 30 * time.Second
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("graceful shutdown complete")
	case <-time.After(shutdownTimeout):
		logger.Warn("shutdown timeout exceeded, forcing exit")
	}
}
