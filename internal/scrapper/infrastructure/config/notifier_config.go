package config

import "time"

type NotifierType string

const (
	NotifierTypeKafka NotifierType = "kafka"
	NotifierTypeHTTP  NotifierType = "http"
)

type NotifierConfig interface {
	Type() NotifierType
}

// HTTPNotifierConfig конфиг для http клиента
type HTTPNotifierConfig struct {
	BotBaseURL string        `env:"BOT_BASE_URL,required"`
	BotTimeout time.Duration `env:"SCRAPPER_TO_BOT_TIMEOUT" envDefault:"5s"`
}

func (c *HTTPNotifierConfig) Type() NotifierType {
	return NotifierTypeHTTP
}

// KafkaNotifierConfig конфиг для Kafka нотификатора
type KafkaNotifierConfig struct {
	Brokers      []string      `env:"KAFKA_BROKERS,required"`
	Topic        string        `env:"KAFKA_UPDATES_TOPIC,required"`
	Compression  string        `env:"KAFKA_COMPRESSION" envDefault:"snappy"`
	BatchSize    int           `env:"KAFKA_BATCH_SIZE" envDefault:"100"`
	BatchTimeout time.Duration `env:"KAFKA_BATCH_TIMEOUT" envDefault:"10ms"`
	RequiredAcks int           `env:"KAFKA_REQUIRED_ACKS" envDefault:"-1"`
}

func (c *KafkaNotifierConfig) Type() NotifierType {
	return NotifierTypeKafka
}
