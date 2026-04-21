package botkafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Service interface {
	HandleUpdate(chatIDs []int64, desc string)
}

type Consumer struct {
	reader  *kafka.Reader
	service Service
	wg      sync.WaitGroup
	stopCh  chan struct{}

	cancel context.CancelFunc
}

func NewConsumer(service Service, brokers []string, topic string, groupID string,
	sessionTimeout time.Duration, minBytes, maxBytes int) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		SessionTimeout: sessionTimeout,
		StartOffset:    kafka.LastOffset,
	})

	return &Consumer{service: service, reader: reader, stopCh: make(chan struct{})}
}

func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		for {
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("read error", "err", err)
				continue
			}

			var update domain.LinkUpdate
			if err := json.Unmarshal(msg.Value, &update); err != nil {
				slog.Error("failed to unmarshal message", "error", err, "key", string(msg.Key))
				continue
			}

			c.service.HandleUpdate(update.TgChatIDs, update.Description)
		}
	}()

	return nil
}

func (c *Consumer) Shutdown(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()

	err := c.reader.Close()
	if err != nil {
		return fmt.Errorf("cannot close reader: %w", err)
	}

	return nil
}
