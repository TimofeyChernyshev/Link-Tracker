package botkafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestConsumer_HandleUpdate(t *testing.T) {
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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := NewMockService(ctrl)

	called := make(chan struct{}, 1)

	upd := domain.LinkUpdate{
		ID:          1,
		URL:         "http://test",
		Description: "updated",
		TgChatIDs:   []int64{1, 2},
	}

	mockService.EXPECT().HandleUpdate(upd.TgChatIDs, upd.Description).Do(func(chatIDs []int64, desc string) {
		called <- struct{}{}
	}).Times(1)

	consumer := NewConsumer(
		mockService,
		brokers,
		topic,
		"",
		10*time.Second,
		1,
		1e6,
	)

	err = consumer.Start(ctx)
	require.NoError(t, err)
	defer consumer.Shutdown(ctx)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: brokers,
		Topic:   topic,
	})
	defer writer.Close()

	data, err := json.Marshal(upd)
	require.NoError(t, err)

	err = writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte("1"),
		Value: data,
	})
	require.NoError(t, err)

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for HandleUpdate")
	}
}
