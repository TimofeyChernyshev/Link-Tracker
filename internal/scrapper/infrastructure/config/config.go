package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ScrapperPort string
	BotBaseURL   string
	AccessType   AccessType
	DBUser       string
	DBPassword   string
	DBHost       string
	DBPort       int
	DBName       string
}

type AccessType string

const (
	AccessTypeSQL AccessType = "sql"
	AccessTypeORM AccessType = "orm"
)

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		slog.Error("PORT is not set in .env.scrapper file")
		return nil, errors.New("bot port not found")
	}

	botURL := os.Getenv("BOT_BASE_URL")
	if botURL == "" {
		slog.Warn("BOT_BASE_URL is not set in .env.scrapper file")
	}

	portString := os.Getenv("DB_PORT")
	dbPort, err := strconv.Atoi(portString)
	if err != nil {
		slog.Warn("DB_PORT is not int", "error", err)
	}

	return &Config{
		ScrapperPort: port,
		BotBaseURL:   botURL,
		AccessType:   AccessType(strings.ToLower(os.Getenv("ACCESS_TYPE"))),
		DBUser:       os.Getenv("DB_USER"),
		DBPassword:   os.Getenv("DB_PASSWORD"),
		DBHost:       os.Getenv("DB_HOST"),
		DBPort:       dbPort,
		DBName:       os.Getenv("DB_NAME"),
	}, nil
}
