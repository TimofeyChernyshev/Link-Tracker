package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ScrapperPort  string
	BotBaseURL    string
	AccessType    AccessType
	DBUser        string
	DBPassword    string
	DBHost        string
	DBPort        int
	DBName        string
	BatchSize     int
	CheckInterval time.Duration
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

	batchSize := getEnvInt("BATCH_SIZE", 100)
	if batchSize < 50 {
		batchSize = 50
	} else if batchSize > 500 {
		batchSize = 500
	}

	var checkInterval time.Duration
	checkIntervalString := os.Getenv("CHECK_INTERVAL")
	if checkIntervalString == "" {
		checkInterval = time.Duration(time.Date(0, 0, 0, 0, 1, 0, 0, time.UTC).Minute())
	} else {
		checkInterval, err = time.ParseDuration(checkIntervalString)
		if err != nil {
			slog.Warn("cannot parse duration config", "error", err)
			checkInterval = time.Duration(time.Date(0, 0, 0, 0, 1, 0, 0, time.UTC).Minute())
		}
	}

	return &Config{
		ScrapperPort:  port,
		BotBaseURL:    botURL,
		AccessType:    AccessType(strings.ToLower(os.Getenv("ACCESS_TYPE"))),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        dbPort,
		DBName:        os.Getenv("DB_NAME"),
		BatchSize:     batchSize,
		CheckInterval: checkInterval,
	}, nil
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
