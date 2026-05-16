package config

import (
	"fmt"
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
	ScrapperPort string `env:"SCRAPPER_PORT,required"`

	// DB
	AccessType AccessType `env:"ACCESS_TYPE,required"`
	DBUser     string     `env:"DB_USER,required"`
	DBPassword string     `env:"DB_PASSWORD,required"`
	DBHost     string     `env:"DB_HOST,required"`
	DBPort     int        `env:"DB_PORT,required"`
	DBName     string     `env:"DB_NAME,required"`

	CheckInterval time.Duration `env:"CHECK_INTERVAL" envDefault:"60s"`

	WorkerCount int `env:"SCRAPPER_WORKER_COUNT" envDefault:"4"`
	BatchSize   int `env:"SCRAPPER_BATCH_SIZE" envDefault:"20"`

	// link checker (GitHub & StackOverflow)
	APIBatchSize       int           `env:"API_BATCH_SIZE" envDefault:"100"`
	LinkCheckerTimeout time.Duration `env:"LINK_CHECKER_TIMEOUT" envDefault:"5s"`
	GighubBaseURL      string        `env:"GITHUB_BASE_URL,required"`
	GithubTimeout      time.Duration `env:"GITHUB_TIMEOUT" envDefault:"5s"`
	StackBaseURL       string        `env:"STACK_BASE_URL,required"`
	StackTimeout       time.Duration `env:"STACK_TIMEOUT" envDefault:"5s"`

	CheckerPreviewLen int `env:"CHECKER_PREVIEW_LEN" envDefault:"200"`

	// HTTP пагинация
	DefaultLimit int `env:"HTTP_DEFAULT_LIMIT" envDefault:"50"`
	MaxLimit     int `env:"HTTP_MAX_LIMIT" envDefault:"100"`

	ShutdownTimeout time.Duration `env:"SCRAPPER_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Kafka/http notifier
	NotificationType NotifierType `env:"NOTIFICATION_TYPE" envDefault:"kafka"`
	NotifierConfig   NotifierConfig

	// Valkey config
	ValkeyAddresses       []string      `env:"VALKEY_ADDRESSES,required"`
	ValkeyPassword        string        `env:"VALKEY_PASSWORD"`
	ValkeyTTL             time.Duration `env:"VALKEY_TTL" envDefault:"300s"`
	ValkeyMaxRetries      int           `env:"VALKEY_MAX_RETRIES" envDefault:"3"`
	ValkeyPoolSize        int           `env:"VALKEY_POOL_SIZE" envDefault:"10"`
	ValkeyMinRetryBackoff time.Duration `env:"VALKEY_MIN_RETRY_BACKOFF" envDefault:"100ms"`
	ValkeyMaxRetryBackoff time.Duration `env:"VALKEY_MAX_RETRY_BACKOFF" envDefault:"1s"`
	ValkeyPingTime        time.Duration `env:"VALKEY_PING_TIME" envDefault:"5s"`
	ValkeyClusterMode     bool          `env:"VALKEY_CLUSTER_MODE" envDefault:"true"`
	ValkeyScanCount       int64         `env:"VALKEY_SCAN_COUNT" envDefault:"100"`
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
		return nil, fmt.Errorf("WORKER_COUNT must be in range [%v;%v]", minWorkers, maxWorkers)
	}

	cfg.AccessType = AccessType(strings.ToLower(string(cfg.AccessType)))

	switch cfg.NotificationType {
	case NotifierTypeKafka:
		var kafkaCfg KafkaNotifierConfig
		if err = env.Parse(&kafkaCfg); err != nil {
			return nil, fmt.Errorf("failed to parse kafka config: %w", err)
		}
		cfg.NotifierConfig = &kafkaCfg

	case NotifierTypeHTTP:
		var httpCfg HTTPNotifierConfig
		if err = env.Parse(&httpCfg); err != nil {
			return nil, fmt.Errorf("failed to parse http config: %w", err)
		}
		cfg.NotifierConfig = &httpCfg

	default:
		return nil, fmt.Errorf("unknown notification type: %s", cfg.NotificationType)
	}

	return &cfg, nil
}
