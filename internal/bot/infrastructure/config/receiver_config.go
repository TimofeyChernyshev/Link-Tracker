package config

import "time"

type ReceiverType string

const (
	ReceiverTypeKafka ReceiverType = "kafka"
	ReceiverTypeHTTP  ReceiverType = "http"
)

type ReceiverConfig interface {
	Type() ReceiverType
}

// HTTPReceiverConfig конфиг для HTTP сервера
type HTTPReceiverConfig struct {
	Port string `env:"BOT_PORT,required"`
}

func (c *HTTPReceiverConfig) Type() ReceiverType {
	return ReceiverTypeHTTP
}

// KafkaReceiverConfig конфиг для Kafka консьюмера
type KafkaReceiverConfig struct {
	Brokers        []string      `env:"KAFKA_BROKERS,required"`
	Topic          string        `env:"KAFKA_UPDATES_TOPIC,required"`
	GroupID        string        `env:"KAFKA_GROUP_ID,required"`
	SessionTimeout time.Duration `env:"KAFKA_SESSION_TIMEOUT" envDefault:"10s"`
	MinBytes       int           `env:"KAFKA_MIN_BYTES" envDefault:"1"`
	MaxBytes       int           `env:"KAFKA_MAX_BYTES" envDefault:"10485760"`

	DLQTopic     string        `env:"KAFKA_DLQ_TOPIC,required"`
	MaxRetries   int           `env:"KAFKA_MAX_RETRIES" envDefault:"3"`
	RetryDelay   time.Duration `env:"KAFKA_RETRY_DELAY" envDefault:"1s"`
	BatchSize    int           `env:"KAFKA_DLQ_BATCH_SIZE" envDefault:"1"`
	BatchTimeout time.Duration `env:"KAFKA_DLQ_BATCH_TIMEOUT" envDefault:"1s"`
}

func (c *KafkaReceiverConfig) Type() ReceiverType {
	return ReceiverTypeKafka
}
