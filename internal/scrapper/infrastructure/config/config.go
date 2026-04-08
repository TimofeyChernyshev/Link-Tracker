package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBatchSize = 100
	defaultBatchSize    = 20
	defaultWorkerCount  = 4
	minAPIBatchSize     = 50
	maxAPIBatchSize     = 500
	minBatchSize        = 10
	maxBatchSize        = 1000
	minWorkers          = 1
	maxWorkers          = 20
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
	APIBatchSize  int
	CheckInterval time.Duration
	WorkerCount   int
	BatchSize     int
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

	APIBatchSize := getEnvInt("API_BATCH_SIZE", defaultAPIBatchSize)
	if APIBatchSize < minAPIBatchSize {
		APIBatchSize = minAPIBatchSize
	} else if APIBatchSize > maxAPIBatchSize {
		APIBatchSize = maxAPIBatchSize
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

	batchSize := getEnvInt("BATCH_SIZE", defaultBatchSize)
	if batchSize < minBatchSize {
		batchSize = minBatchSize
	} else if batchSize > maxBatchSize {
		batchSize = maxBatchSize
	}

	workerCount := getEnvInt("WORKER_COUNT", defaultWorkerCount)
	if workerCount < minWorkers {
		workerCount = minWorkers
	} else if workerCount > maxWorkers {
		workerCount = maxWorkers
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
		APIBatchSize:  APIBatchSize,
		CheckInterval: checkInterval,
		WorkerCount:   workerCount,
		BatchSize:     batchSize,
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
