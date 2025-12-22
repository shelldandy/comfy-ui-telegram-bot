package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
	ComfyUI  ComfyUIConfig  `mapstructure:"comfyui"`
	Image    ImageConfig    `mapstructure:"image"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type TelegramConfig struct {
	BotToken       string        `mapstructure:"bot_token"`
	AllowedUsers   []int64       `mapstructure:"allowed_users"`
	PollingTimeout int           `mapstructure:"polling_timeout"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

type ComfyUIConfig struct {
	BaseURL      string        `mapstructure:"base_url"`
	WebSocketURL string        `mapstructure:"websocket_url"`
	WorkflowPath string        `mapstructure:"workflow_path"`
	Timeout      time.Duration `mapstructure:"timeout"`
}

type ImageConfig struct {
	JPEGQuality int `mapstructure:"jpeg_quality"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	JSONFormat bool   `mapstructure:"json_format"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("telegram.polling_timeout", 60)
	v.SetDefault("telegram.request_timeout", "5m")
	v.SetDefault("comfyui.base_url", "http://localhost:8188")
	v.SetDefault("comfyui.websocket_url", "ws://localhost:8188/ws")
	v.SetDefault("comfyui.timeout", "5m")
	v.SetDefault("image.jpeg_quality", 80)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.json_format", false)

	// Config file locations
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/comfy-tg-bot")

	// Environment variables
	v.SetEnvPrefix("COMFY_BOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (optional)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
		// Config file not found is OK, use env vars and defaults
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Telegram.BotToken == "" {
		return fmt.Errorf("telegram.bot_token is required")
	}
	if len(c.Telegram.AllowedUsers) == 0 {
		return fmt.Errorf("telegram.allowed_users must contain at least one user ID")
	}
	if c.ComfyUI.WorkflowPath == "" {
		return fmt.Errorf("comfyui.workflow_path is required")
	}
	if c.Image.JPEGQuality < 1 || c.Image.JPEGQuality > 100 {
		return fmt.Errorf("image.jpeg_quality must be between 1 and 100")
	}
	return nil
}
