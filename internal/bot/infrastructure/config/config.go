package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	TelegramToken    string `env:"TELEGRAM_TOKEN,required"`
	TelegramEndpoint string `env:"TELEGRAM_API_URL"`

	BotPort         string        `env:"BOT_PORT,required"`
	ScrapperBaseURL string        `env:"SCRAPPER_BASE_URL,required"`
	ScrapperTimeout time.Duration `env:"BOT_TO_SCRAPPER_TIMEOUT" envDefault:"5s"`

	ReceiverType        string        `env:"RECEIVER_TYPE" envDefault:"kafka"`
	KafkaBrokers        []string      `env:"KAFKA_BROKERS"`
	KafkaTopic          string        `env:"KAFKA_TOPIC"`
	KafkaGroupID        string        `env:"KAFKA_GROUP_ID"`
	KafkaSessionTimeout time.Duration `env:"KAFKA_SESSION_TIMEOUT" envDefault:"30s"`

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
}

func Load() (*Config, error) {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.ReceiverType == "kafka" {
		if cfg.KafkaBrokers == nil {
			return nil, fmt.Errorf("KAFKA_BROKERS is required when RECEIVER_TYPE=kafka")
		}
		if cfg.KafkaTopic == "" {
			return nil, fmt.Errorf("KAFKA_TOPIC is required when RECEIVER_TYPE=kafka")
		}
		if cfg.KafkaGroupID == "" {
			return nil, fmt.Errorf("KAFKA_GROUP_ID is required when RECEIVER_TYPE=kafka")
		}
	}

	return &cfg, nil
}
