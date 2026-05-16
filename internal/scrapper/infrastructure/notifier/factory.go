package scrappernotifier

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	httpnotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier/http"
	kafkanotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier/kafka"
)

type Notifier interface {
	SendUpdate(ctx context.Context, upd domain.LinkUpdate) error
}

func NewNotifier(notifierConfig config.NotifierConfig) (Notifier, error) {
	switch cfg := notifierConfig.(type) {
	case *config.HTTPNotifierConfig:
		return httpnotifier.NewBotClient(cfg.BotBaseURL, cfg.BotTimeout), nil
	case *config.KafkaNotifierConfig:
		return kafkanotifier.NewKafkaNotifier(
			cfg.Topic, cfg.Compression, cfg.Brokers,
			cfg.BatchSize, cfg.RequiredAcks, cfg.BatchTimeout,
		), nil
	default:
		return nil, fmt.Errorf("unknown notifier type: %s", notifierConfig.Type())
	}
}
