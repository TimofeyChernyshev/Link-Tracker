package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

type Config struct {
	ShutdownTimeout time.Duration `env:"AGENT_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	CommonKafkaConfig   config.CommonConfig
	DLQConfig           config.DLQConfig           `envPrefix:"AGENT_KAFKA_"`
	KafkaConsumerConfig config.KafkaConsumerConfig `envPrefix:"AGENT_KAFKA_"`
	KafkaNotifierConfig config.KafkaNotifierConfig `envPrefix:"AGENT_KAFKA_"`

	FilterStopWords       []string `env:"FILTER_STOP_WORDS"`
	FilterExcludedAuthors []string `env:"FILTER_EXCLUDED_AUTHORS"`
	FilterMinLength       int      `env:"FILTER_MIN_LENGTH" envDefault:"20"`

	SummarizationThreshold int `env:"SUMMARIZATION_THRESHOLD" envDefault:"500"`
}

func Load() (*Config, error) {
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
