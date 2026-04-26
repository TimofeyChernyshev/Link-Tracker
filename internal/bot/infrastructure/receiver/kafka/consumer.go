package botkafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Service interface {
	HandleUpdate(chatIDs []int64, desc string) error
}

type DeadLetterMessage struct {
	OriginalMessage json.RawMessage `json:"original_message"`
	Key             string          `json:"key"`
	Topic           string          `json:"topic"`
	Partition       int             `json:"partition"`
	Offset          int64           `json:"offset"`
	ErrorReason     string          `json:"error_reason"`
	ErrorType       string          `json:"error_type"`
	Retries         int             `json:"retries"`
	Timestamp       time.Time       `json:"timestamp"`
}

type Consumer struct {
	reader     *kafka.Reader
	service    Service
	wg         sync.WaitGroup
	stopCh     chan struct{}
	dlqWriter  *kafka.Writer
	maxRetries int
	retryDelay time.Duration

	cancel context.CancelFunc
}

func NewConsumer(service Service, brokers []string, topic string, groupID string,
	sessionTimeout time.Duration, minBytes, maxBytes int,
	maxRetries, batchSize int, retryDelay, batchTimeout time.Duration, dlqTopic string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		SessionTimeout: sessionTimeout,
		StartOffset:    kafka.FirstOffset,
	})

	var DLQWriter *kafka.Writer
	if dlqTopic != "" {
		DLQWriter = &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        dlqTopic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			BatchSize:    batchSize,
			BatchTimeout: batchTimeout,
		}
	}

	return &Consumer{
		service: service, reader: reader, stopCh: make(chan struct{}),
		dlqWriter: DLQWriter, maxRetries: maxRetries, retryDelay: retryDelay,
	}
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
			if err = json.Unmarshal(msg.Value, &update); err != nil {
				slog.Error("failed to unmarshal message", "error", err, "key", string(msg.Key))
				c.sendToDLQ(msg, err, "unmarshal", 0)
				continue
			}

			attempt := 0

			for attempt = range c.maxRetries + 1 {
				if attempt > 0 {
					slog.Warn("retrying to handle message", "attempt", attempt)
					time.Sleep(c.retryDelay)
				}

				err = c.service.HandleUpdate(update.TgChatIDs, update.Description)
				if err == nil {
					break
				}
			}

			if attempt == c.maxRetries+1 {
				slog.Error("handle update error", "error", err, "attempt", attempt)
				c.sendToDLQ(msg, errors.New("all retries exhausted"), "processing", c.maxRetries)
			}
		}
	}()

	return nil
}

func (c *Consumer) Shutdown(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()

	var errors []error

	err := c.reader.Close()
	if err != nil {
		errors = append(errors, fmt.Errorf("cannot close reader: %w", err))
	}

	if c.dlqWriter != nil {
		err = c.dlqWriter.Close()
		if err != nil {
			errors = append(errors, fmt.Errorf("cannot close DLQ writer: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

func (c *Consumer) sendToDLQ(msg kafka.Message, reason error, errorType string, retries int) {
	if c.dlqWriter == nil {
		slog.Warn("DLQ not configured, lost message", "key", string(msg.Key), "reason", reason.Error())
		return
	}

	dlqMsg := DeadLetterMessage{
		OriginalMessage: msg.Value,
		Key:             string(msg.Key),
		Topic:           msg.Topic,
		Partition:       msg.Partition,
		Offset:          msg.Offset,
		ErrorReason:     reason.Error(),
		ErrorType:       errorType,
		Retries:         retries,
		Timestamp:       time.Now(),
	}

	data, err := json.Marshal(dlqMsg)
	if err != nil {
		slog.Error("cannot marshal DLQ message", "error", err)
		return
	}

	dlqKafkaMsg := kafka.Message{
		Key:   msg.Key,
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "original-topic", Value: []byte(msg.Topic)},
			{Key: "error-type", Value: []byte(errorType)},
			{Key: "error-reason", Value: []byte(reason.Error())},
			{Key: "retries", Value: []byte(strconv.Itoa(retries))},
		},
	}

	if err := c.dlqWriter.WriteMessages(context.Background(), dlqKafkaMsg); err != nil {
		slog.Error("cannot send to DLQ", "error", err)
		return
	}
}
