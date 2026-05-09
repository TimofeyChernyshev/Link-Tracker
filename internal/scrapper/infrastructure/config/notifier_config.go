package config

import (
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

// HTTPBotNotifierConfig конфиг для http клиента
type HTTPBotNotifierConfig struct {
	BaseURL              string                          `env:"BOT_BASE_URL,required"`
	RetryConfig          resilience.RetryConfig          `envPrefix:"BASIC_"`
	CircuitBreakerConfig resilience.CircuitBreakerConfig `envPrefix:"BASIC_CB_"`
	RateLimit            int                             `env:"BOT_RATE_LIMIT" envDefault:"100"`
	Timeout              time.Duration                   `env:"BOT_TIMEOUT" envDefault:"5s"`
}

// KafkaBotNotifierConfig конфиг для Kafka нотификатора
type KafkaBotNotifierConfig struct {
	Brokers      []string      `env:"KAFKA_BROKERS,required"`
	Topic        string        `env:"KAFKA_UPDATES_TOPIC,required"`
	Compression  string        `env:"KAFKA_COMPRESSION" envDefault:"snappy"`
	BatchSize    int           `env:"KAFKA_BATCH_SIZE" envDefault:"100"`
	BatchTimeout time.Duration `env:"KAFKA_BATCH_TIMEOUT" envDefault:"10ms"`
	RequiredAcks int           `env:"KAFKA_REQUIRED_ACKS" envDefault:"-1"`
}
