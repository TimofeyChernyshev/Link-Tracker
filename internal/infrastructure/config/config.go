package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
}

func Load() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		slog.Error(".env file not found")
		return nil, err
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		slog.Error("TELEGRAM_TOKEN is not set in .env file")
		return nil, errors.New("token not found")
	}

	return &Config{TelegramToken: token}, nil
}
