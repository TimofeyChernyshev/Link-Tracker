package kafkanotifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type KafkaNotifier struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaNotifier(topic, compression string, brokers []string, batchSize, requiredAcks int, batchTimeout time.Duration) *KafkaNotifier {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		RequiredAcks: kafka.RequiredAcks(requiredAcks),
		Compression:  getCompression(compression),
	}

	return &KafkaNotifier{
		writer: writer,
		topic:  topic,
	}
}

func (n *KafkaNotifier) SendUpdate(ctx context.Context, upd domain.LinkUpdate) error {
	linkUpdate := linkUpdateFromDomain(upd)
	data, err := json.Marshal(linkUpdate)
	if err != nil {
		slog.Error("cannot marshal update", "error", err)
		return fmt.Errorf("cannot marshal update: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(strconv.FormatInt(linkUpdate.ID, 10)),
		Value: data,
		Time:  time.Now(),
	}

	if err = n.writer.WriteMessages(ctx, msg); err != nil {
		slog.Error("cannot send message to kafka", "error", err)
		return fmt.Errorf("cannot send message: %w", err)
	}

	return nil
}

func (n *KafkaNotifier) Close() error {
	return n.writer.Close()
}

func getCompression(c string) kafka.Compression {
	switch c {
	case "gzip":
		return kafka.Gzip
	case "snappy":
		return kafka.Snappy
	case "lz4":
		return kafka.Lz4
	case "zstd":
		return kafka.Zstd
	default:
		return kafka.Snappy
	}
}
