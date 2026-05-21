package config

import "time"

type HTTPReceiverConfig struct {
	Port string `env:"PORT,required"`
}

type KafkaReceiverConfig struct {
	Brokers        []string      `env:"BROKERS,required"`
	Topic          string        `env:"UPDATES_TOPIC,required"`
	GroupID        string        `env:"GROUP_ID,required"`
	SessionTimeout time.Duration `env:"SESSION_TIMEOUT" envDefault:"10s"`
	MinBytes       int           `env:"MIN_BYTES" envDefault:"1"`
	MaxBytes       int           `env:"MAX_BYTES" envDefault:"10485760"`

	DLQTopic     string        `env:"DLQ_TOPIC,required"`
	MaxRetries   int           `env:"MAX_RETRIES" envDefault:"3"`
	RetryDelay   time.Duration `env:"RETRY_DELAY" envDefault:"1s"`
	BatchSize    int           `env:"DLQ_BATCH_SIZE" envDefault:"1"`
	BatchTimeout time.Duration `env:"DLQ_BATCH_TIMEOUT" envDefault:"1s"`
}
