package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

type Config struct {
	TelegramToken    string `env:"TELEGRAM_TOKEN,required"`
	TelegramEndpoint string `env:"TELEGRAM_API_URL"`

	ScrapperBaseURL              string                          `env:"SCRAPPER_BASE_URL,required"`
	ScrapperRetryConfig          resilience.RetryConfig          `envPrefix:"SCRAPPER_"`
	ScrapperCircuitBreakerConfig resilience.CircuitBreakerConfig `envPrefix:"SCRAPPER_CB_"`
	ScrapperRateLimit            int                             `env:"SCRAPPER_RATE_LIMIT" envDefault:"100"`
	ScrapperTimeout              time.Duration                   `env:"SCRAPPER_TIMEOUT" envDefault:"5s"`

	TimeoutCheckLink      time.Duration `env:"TIMEOUT_CHECK_LINK" envDefault:"10s"`
	TimeoutSaveLink       time.Duration `env:"TIMEOUT_SAVE_LINK" envDefault:"5s"`
	TimeoutStartHandler   time.Duration `env:"TIMEOUT_START_HANDLER" envDefault:"5s"`
	TimeoutUntrackHandler time.Duration `env:"TIMEOUT_UNTRACK_HANDLER" envDefault:"5s"`
	TimeoutListHandler    time.Duration `env:"TIMEOUT_LIST_HANDLER" envDefault:"5s"`

	WorkerCount        int `env:"BOT_WORKER_COUNT" envDefault:"8"`
	SenderCount        int `env:"BOT_SENDER_COUNT" envDefault:"4"`
	JobsBufferSize     int `env:"BOT_JOBS_BUFFER_SIZE" envDefault:"100"`
	OutgoingBufferSize int `env:"BOT_OUTGOING_BUFFER_SIZE" envDefault:"100"`

	ShutdownTimeout time.Duration `env:"BOT_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Rate limiter для HTTP сервера
	RateLimiterConfig resilience.RateLimiterConfig `envPrefix:"BOT_RATE_LIMITER_"`

	// Port где расположен Bot
	BotPort string `env:"BOT_PORT,required"`

	// Конфиг для Kafka
	CommonKafkaConfig   config.CommonConfig
	DLQConfig           config.DLQConfig           `envPrefix:"BOT_KAFKA_"`
	KafkaConsumerConfig config.KafkaConsumerConfig `envPrefix:"BOT_KAFKA_"`

	BotMetricTick time.Duration `env:"BOT_MEMORY_METRIC_TICK" envDefault:"30s"`
}

func Load() (*Config, error) {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
