package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type ConsumerSuite struct {
	suite.Suite

	ctrl           *gomock.Controller
	kafkaContainer *kafka.KafkaContainer
	topic          string
	dlqTopic       string
	ctx            context.Context
	service        *MockService
	brokers        []string

	consumer *Consumer
}

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	kafkaContainer, err := kafka.Run(s.ctx, "confluentinc/cp-kafka:7.5.0", kafka.WithClusterID("test-cluster"))
	s.Require().NoError(err)

	brokers, err := kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(brokers)
	s.brokers = brokers

	s.topic = "link.raw-updates"
	s.dlqTopic = "link.raw-updates-dlq"

	s.Require().NoError(err)
}

func (s *ConsumerSuite) TearDownSuite() {
	if s.kafkaContainer != nil {
		err := s.kafkaContainer.Terminate(s.ctx)
		s.Require().NoError(err)
	}
}

func (s *ConsumerSuite) SetupTest() {
	s.createTopic(s.topic)
	s.createTopic(s.dlqTopic)

	s.ctrl = gomock.NewController(s.T())
	s.service = NewMockService(s.ctrl)

	uniqueGroupID := fmt.Sprintf("test-group-%d", time.Now().UnixNano())

	s.consumer = NewConsumer(
		s.service,
		s.brokers,
		s.topic,
		uniqueGroupID,
		10*time.Second,
		1,
		1e6,
		3,
		1,
		time.Second,
		time.Second,
		s.dlqTopic,
	)

	err := s.consumer.Start(s.ctx)
	s.Require().NoError(err)
}

func (s *ConsumerSuite) TearDownTest() {
	if s.consumer != nil {
		err := s.consumer.Shutdown(s.ctx)
		s.Require().NoError(err)
	}

	s.ctrl.Finish()
}

func (s *ConsumerSuite) TestConsumer_HandleUpdate() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "updated",
		Author:      "123",
		TgChatIDs:   []int64{1, 2},
	}

	called := make(chan bool, 1)

	s.service.EXPECT().HandleRawUpdate(upd).Return(nil).DoAndReturn(
		func(_ domain.RawUpdate) error {
			called <- true
			return nil
		},
	)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: s.brokers,
		Topic:   s.topic,
	})
	defer writer.Close()

	data, err := json.Marshal(upd)
	s.Require().NoError(err)

	err = writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte("1"),
		Value: data,
	})
	s.Require().NoError(err)

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		s.Fail("timeout waiting for HandleRawUpdate")
	}
}

func (s *ConsumerSuite) TestConsumer_WrongMessageFormat() {
	data := []byte(`{invalid json`)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: s.brokers,
		Topic:   s.topic,
	})
	defer writer.Close()

	err := writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte("1"),
		Value: data,
	})
	s.Require().NoError(err)

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     s.brokers,
		Topic:       s.dlqTopic,
		MinBytes:    1,
		MaxBytes:    1e6,
		StartOffset: kafkago.FirstOffset,
	})
	defer reader.Close()

	msg, err := reader.ReadMessage(s.ctx)
	s.Require().NoError(err)

	var dlqMsg DeadLetterMessage
	err = json.Unmarshal(msg.Value, &dlqMsg)
	s.Require().NoError(err)

	s.Equal("unmarshal", dlqMsg.ErrorType)
	s.Equal("link.raw-updates", dlqMsg.Topic)
}

func (s *ConsumerSuite) createTopic(topic string) {
	conn, err := kafkago.Dial("tcp", s.brokers[0])
	s.Require().NoError(err)
	defer conn.Close()

	_ = conn.DeleteTopics(topic)

	err = conn.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     3,
		ReplicationFactor: 1,
	})
	s.Require().NoError(err)
}
