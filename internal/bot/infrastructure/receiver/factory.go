package receiver

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	bothttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/http"
	botkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/kafka"
)

type Receiver interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

func NewReceiver(receiverConfig config.ReceiverConfig, service bothttp.Service) (Receiver, error) {
	switch cfg := receiverConfig.(type) {
	case *config.HTTPReceiverConfig:
		return bothttp.NewServer(service, cfg.Port), nil
	case *config.KafkaReceiverConfig:
		return botkafka.NewConsumer(
			service, cfg.Brokers, cfg.Topic, cfg.GroupID,
			cfg.SessionTimeout, cfg.MinBytes, cfg.MaxBytes,
			cfg.MaxRetries, cfg.BatchSize, cfg.RetryDelay, cfg.BatchTimeout, cfg.DLQTopic,
		), nil
	default:
		return nil, fmt.Errorf("unknown receiver type: %s", receiverConfig.Type())
	}
}
