package config

import (
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
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
	CommonConfig   config.CommonConfig
	NotifierConfig config.KafkaNotifierConfig `envPrefix:"SCRAPPER_"`
}
