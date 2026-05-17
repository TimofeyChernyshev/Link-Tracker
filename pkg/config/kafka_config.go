package config

import "time"

// CommonConfig - общие настройки Kafka
type CommonConfig struct {
	Brokers []string `env:"KAFKA_BROKERS,required"`
}

type DLQConfig struct {
	DLQTopic     string        `env:"DLQ_TOPIC,required"`
	MaxRetries   int           `env:"MAX_RETRIES" envDefault:"3"`
	RetryDelay   time.Duration `env:"RETRY_DELAY" envDefault:"1s"`
	BatchSize    int           `env:"DLQ_BATCH_SIZE" envDefault:"1"`
	BatchTimeout time.Duration `env:"DLQ_BATCH_TIMEOUT" envDefault:"1s"`
}

type KafkaConsumerConfig struct {
	Topic          string        `env:"CONSUMER_TOPIC,required"`
	GroupID        string        `env:"GROUP_ID,required"`
	SessionTimeout time.Duration `env:"SESSION_TIMEOUT" envDefault:"10s"`
	MinBytes       int           `env:"MIN_BYTES" envDefault:"1"`
	MaxBytes       int           `env:"MAX_BYTES" envDefault:"10485760"`
}

type KafkaNotifierConfig struct {
	Topic        string        `env:"NOTIFIER_TOPIC,required"`
	Compression  string        `env:"NOTIFIER_COMPRESSION" envDefault:"snappy"`
	BatchSize    int           `env:"NOTIFIER_BATCH_SIZE" envDefault:"100"`
	BatchTimeout time.Duration `env:"NOTIFIER_BATCH_TIMEOUT" envDefault:"10ms"`
	RequiredAcks int           `env:"NOTIFIER_REQUIRED_ACKS" envDefault:"-1"`
}
