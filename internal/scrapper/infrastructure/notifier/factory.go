package scrappernotifier

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	httpnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier/http"
	kafkanotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier/kafka"
)

type Notifier interface {
	SendUpdate(ctx context.Context, upd domain.LinkUpdate) error
}

func NewNotifier(notifierType string,
	botBaseURL string, botTimeout time.Duration,
	kafkaTopic, kafkaCompression string, kafkaBrokers []string,
	kafkaBatchSize, kafkaRequiredAcks int, kafkaBatchTimeout time.Duration,
) (Notifier, error) {
	switch notifierType {
	case "http":
		return httpnotifier.NewBotClient(botBaseURL, botTimeout), nil
	case "kafka":
		return kafkanotifier.NewKafkaNotifier(
			kafkaTopic, kafkaCompression, kafkaBrokers,
			kafkaBatchSize, kafkaRequiredAcks, kafkaBatchTimeout,
		), nil
	default:
		return nil, fmt.Errorf("unknown notifier type: %s", notifierType)
	}
}
