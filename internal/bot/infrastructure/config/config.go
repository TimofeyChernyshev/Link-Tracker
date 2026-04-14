package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	TelegramToken       string        `env:"TELEGRAM_TOKEN,required"`
	TelegramEndpoint    string        `env:"TELEGRAM_API_URL"`
	BotPort             string        `env:"PORT,required"`
	ScrapperBaseURL     string        `env:"SCRAPPER_BASE_URL,required"`
	ReceiverType        string        `env:"RECEIVER_TYPE" envDefault:"kafka"`
	KafkaBrokers        []string      `env:"KAFKA_BROKERS"`
	KafkaTopic          string        `env:"KAFKA_TOPIC"`
	KafkaGroupID        string        `env:"KAFKA_GROUP_ID"`
	KafkaSessionTimeout time.Duration `env:"KAFKA_SESSION_TIMEOUT" envDefault:"30s"`
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
