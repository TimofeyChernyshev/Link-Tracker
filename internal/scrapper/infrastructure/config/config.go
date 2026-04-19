package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

const (
	minAPIBatchSize = 50
	maxAPIBatchSize = 500
	minBatchSize    = 10
	maxBatchSize    = 1000
	minWorkers      = 1
	maxWorkers      = 20
)

type Config struct {
	ScrapperPort string        `env:"PORT,required"`
	BotBaseURL   string        `env:"BOT_BASE_URL,required"`
	BotTimeout   time.Duration `env:"BOT_TIMEOUT" envDefault:"5s"`

	AccessType AccessType `env:"ACCESS_TYPE,required"`
	DBUser     string     `env:"DB_USER,required"`
	DBPassword string     `env:"DB_PASSWORD,required"`
	DBHost     string     `env:"DB_HOST,required"`
	DBPort     int        `env:"DB_PORT,required"`
	DBName     string     `env:"DB_NAME,required"`

	CheckInterval time.Duration `env:"CHECK_INTERVAL" envDefault:"60s"`

	WorkerCount int `env:"WORKER_COUNT" envDefault:"4"`
	BatchSize   int `env:"BATCH_SIZE" envDefault:"20"`

	APIBatchSize       int           `env:"API_BATCH_SIZE" envDefault:"100"`
	LinkCheckerTimeout time.Duration `env:"LINK_CHECKER_TIMEOUT" envDefault:"5s"`
	GighubBaseURL      string        `env:"GITHUB_BASE_URL,required"`
	GithubTimeout      time.Duration `env:"GITHUB_TIMEOUT" envDefault:"5s"`
	StackBaseURL       string        `env:"STACK_BASE_URL,required"`
	StackTimeout       time.Duration `env:"STACK_TIMEOUT" envDefault:"5s"`

	CheckerPreviewLen int `env:"CHECKER_PREVIEW_LEN" envDefault:"200"`
}

type AccessType string

const (
	AccessTypeSQL AccessType = "sql"
	AccessTypeORM AccessType = "orm"
)

func Load() (*Config, error) {
	var cfg Config

	err := env.Parse(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.APIBatchSize < minAPIBatchSize || cfg.APIBatchSize > maxAPIBatchSize {
		return nil, fmt.Errorf("API_BATCH_SIZE must be in range [%v;%v]", minAPIBatchSize, maxAPIBatchSize)
	}

	if cfg.BatchSize < minBatchSize || cfg.BatchSize > maxBatchSize {
		return nil, fmt.Errorf("BATCH_SIZE must be in range [%v;%v]", minBatchSize, maxBatchSize)
	}

	if cfg.WorkerCount < minWorkers || cfg.WorkerCount > maxWorkers {
		slog.Error("wc", "wc", cfg.WorkerCount)
		return nil, fmt.Errorf("WORKER_COUNT must be in range [%v;%v]", minWorkers, maxWorkers)
	}

	cfg.AccessType = AccessType(strings.ToLower(string(cfg.AccessType)))

	return &cfg, nil
}
