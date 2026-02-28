package config

import (
	"errors"
	"log/slog"
	"os"
)

type Config struct {
	TelegramToken string
	BotHTTPPort   string
}

func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		slog.Error("TELEGRAM_TOKEN is not set in .env file")
		return nil, errors.New("token not found")
	}

	port := os.Getenv("BOT_HTTP_PORT")
	if port == "" {
		slog.Error("BOT_HTTP_PORT is not set in .env file")
		return nil, errors.New("bot port not found")
	}

	return &Config{TelegramToken: token, BotHTTPPort: port}, nil
}
