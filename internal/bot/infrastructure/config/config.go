package config

import (
	"errors"
	"log/slog"
	"os"
)

type Config struct {
	TelegramToken   string
	BotPort         string
	ScrapperBaseURL string
}

func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		slog.Error("TELEGRAM_TOKEN is not set in .env.bot file")
		return nil, errors.New("token not found")
	}

	port := os.Getenv("PORT")
	if port == "" {
		slog.Error("PORT is not set in .env.bot file")
		return nil, errors.New("bot port not found")
	}

	scrapperURL := os.Getenv("SCRAPPER_BASE_URL")
	if scrapperURL == "" {
		slog.Error("SCRAPPER_BASE_URL is not set in .env.bot file")
		return nil, errors.New("scrapper base url not found")
	}

	return &Config{TelegramToken: token, BotPort: port, ScrapperBaseURL: scrapperURL}, nil
}
