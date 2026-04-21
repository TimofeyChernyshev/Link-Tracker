package receiver

import (
	"context"
	"fmt"
	"time"

	bothttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/http"
	botkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/kafka"
)

type Receiver interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type ReceiverFactory struct {
}

func NewReceiverFactory() *ReceiverFactory {
	return &ReceiverFactory{}
}

func CreateReceiver(receiverType string,
	service bothttp.Service, brokers []string, topic string, groupID string, sessionTimeout time.Duration,
	minBytes, maxBytes int, httpPort string,
) (Receiver, error) {
	switch receiverType {
	case "kafka":
		return botkafka.NewConsumer(service, brokers, topic, groupID, sessionTimeout, minBytes, maxBytes), nil
	case "http":
		return bothttp.NewServer(service, httpPort), nil
	default:
		return nil, fmt.Errorf("unknown receiver type: %s", receiverType)
	}
}
