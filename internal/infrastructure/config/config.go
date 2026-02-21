package config

import (
	"errors"
	"log/slog"
	"os"
)

type Config struct {
	TelegramToken string
}

func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		slog.Error("TELEGRAM_TOKEN is not set in .env file")
		return nil, errors.New("token not found")
	}

	return &Config{TelegramToken: token}, nil
}
