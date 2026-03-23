package config

import (
	"errors"
	"log/slog"
	"os"
)

type Config struct {
	ScrapperPort string
	BotBaseURL   string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		slog.Error("PORT is not set in .env.scrapper file")
		return nil, errors.New("bot port not found")
	}

	botURL := os.Getenv("BOT_BASE_URL")
	if botURL == "" {
		slog.Error("BOT_BASE_URL is not set in .env.scrapper file")
	}

	return &Config{ScrapperPort: port, BotBaseURL: botURL}, nil
}
