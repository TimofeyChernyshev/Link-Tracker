package kafkanotifier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestKafkaNotifier_SendUpdate(t *testing.T) {
	ctx := context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(t)

	kafkaContainer, err := kafka.Run(ctx, "confluentinc/cp-kafka:7.5.0", kafka.WithClusterID("test-cluster"))
	require.NoError(t, err)
	defer kafkaContainer.Terminate(ctx)

	brokers, err := kafkaContainer.Brokers(ctx)
	require.NoError(t, err)

	topic := "test-topic"

	conn, err := kafkago.Dial("tcp", brokers[0])
	require.NoError(t, err)

	require.NoError(t, conn.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}))
	conn.Close()

	notifier := NewKafkaNotifier(
		topic,
		"snappy",
		brokers,
		1,
		-1,
		time.Second,
	)
	defer notifier.Close()

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 1e6,
	})
	defer reader.Close()

	upd := domain.LinkUpdate{
		ID:          1,
		URL:         "http://test",
		Description: "updated",
		ChatIDs:     []int64{1, 2},
	}

	err = notifier.SendUpdate(ctx, upd)
	require.NoError(t, err)

	msg, err := reader.ReadMessage(ctx)
	require.NoError(t, err)

	var actual domain.LinkUpdate
	err = json.Unmarshal(msg.Value, &actual)
	require.NoError(t, err)

	require.Equal(t, upd.ID, actual.ID)
	require.Equal(t, upd.URL, actual.URL)
}
