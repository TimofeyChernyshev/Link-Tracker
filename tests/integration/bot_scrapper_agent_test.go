package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BotScrapperAgentSuite struct {
	suite.Suite
	ctx          context.Context
	fakeTelegram *httptest.Server
	network      *testcontainers.DockerNetwork
	scrapper     testcontainers.Container
	agent        testcontainers.Container
	bot          testcontainers.Container
	scrapperURL  string
	botURL       string
	lastUpdateID int
	userMessages []map[string]interface{}

	lastBotMessage    string
	rawTopic          string
	processedTopic    string
	dlqRawTopic       string
	dlqProcessedTopic string
	kafkaContainer    *kafka.KafkaContainer

	postgresContainer testcontainers.Container

	valkeyNodes []testcontainers.Container
	valkeyInit  testcontainers.Container
}

func TestBotScrapperAgentSuite(t *testing.T) {
	suite.Run(t, new(BotScrapperAgentSuite))
}

func (s *BotScrapperAgentSuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	s.userMessages = make([]map[string]interface{}, 0)
	s.lastUpdateID = 0
	s.rawTopic = "link.raw-updates"
	s.processedTopic = "link.processed-updates"
	s.dlqRawTopic = "link.raw-updates-dlq"
	s.dlqProcessedTopic = "link.processed-updates-dlq"

	s.fakeTelegram = s.StartFakeTelegram()

	u, _ := url.Parse(s.fakeTelegram.URL)
	telegramURL := strings.Replace(s.fakeTelegram.URL, u.Hostname(), "host.docker.internal", 1)

	net, err := network.New(s.ctx)
	s.Require().NoError(err)
	s.network = net

	postgresContainer, err := StartPostgres(s.ctx, net.Name)
	s.Require().NoError(err)
	s.postgresContainer = postgresContainer

	kafkaContainer, err := kafka.Run(
		s.ctx,
		"confluentinc/cp-kafka:7.5.0",
		kafka.WithClusterID("test-cluster"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("started").
				WithOccurrence(1).
				WithStartupTimeout(90*time.Second),
		),
		network.WithNetwork([]string{"kafka"}, s.network),
	)
	s.Require().NoError(err)
	s.kafkaContainer = kafkaContainer

	internalBroker := "kafka:9092"

	valkeyNodes, err := StartValkeyNode(s.ctx, net.Name)
	s.Require().NoError(err)
	s.valkeyNodes = valkeyNodes

	valkeyInit, err := InitValkeyCluster(s.ctx, net.Name)
	s.Require().NoError(err)
	s.valkeyInit = valkeyInit

	s.Eventually(func() bool {
		return IsValkeyClusterReady(s.ctx, net.Name)
	}, 20*time.Second, 500*time.Millisecond)

	brokers, err := kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(brokers)

	conn, err := kafkago.Dial("tcp", brokers[0])
	s.Require().NoError(err)
	defer conn.Close()

	err = conn.CreateTopics(
		kafkago.TopicConfig{
			Topic:             s.rawTopic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
		kafkago.TopicConfig{
			Topic:             s.processedTopic,
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
		kafkago.TopicConfig{
			Topic:             s.dlqRawTopic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
		kafkago.TopicConfig{
			Topic:             s.dlqProcessedTopic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	)
	s.Require().NoError(err)

	// Scrapper с отправкой в raw-updates
	scrapper, scrapperURL, err := StartScrapperWithKafka(s.ctx, net.Name, s.rawTopic, []string{internalBroker})
	s.Require().NoError(err)
	s.scrapper = scrapper
	s.scrapperURL = scrapperURL

	// AI Agent
	agent, err := StartAgent(s.ctx, net.Name, s.rawTopic, s.processedTopic, s.dlqRawTopic, []string{internalBroker})
	s.Require().NoError(err)
	s.agent = agent

	// Bot с чтением из processed-updates
	bot, botURL, err := StartBotWithKafka(s.ctx, net.Name, telegramURL, s.processedTopic, s.dlqProcessedTopic, []string{internalBroker})
	s.Require().NoError(err)
	s.bot = bot
	s.botURL = botURL
}

func (s *BotScrapperAgentSuite) TearDownSuite() {
	if s.bot != nil {
		s.bot.Terminate(s.ctx)
	}
	if s.agent != nil {
		s.agent.Terminate(s.ctx)
	}
	if s.scrapper != nil {
		s.scrapper.Terminate(s.ctx)
	}
	if s.valkeyInit != nil {
		s.valkeyInit.Terminate(s.ctx)
	}
	for _, n := range s.valkeyNodes {
		n.Terminate(s.ctx)
	}
	if s.kafkaContainer != nil {
		s.kafkaContainer.Terminate(s.ctx)
	}
	if s.postgresContainer != nil {
		s.postgresContainer.Terminate(s.ctx)
	}
	if s.network != nil {
		s.network.Remove(s.ctx)
	}
	if s.fakeTelegram != nil {
		s.fakeTelegram.Close()
	}
}

func (s *BotScrapperAgentSuite) StartFakeTelegram() *httptest.Server {
	handler := http.NewServeMux()

	handler.HandleFunc("/bottest/getMe", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{
			"ok": true,
			"result": {
				"id": 123456,
				"is_bot": true,
				"username": "test_bot"
			}
		}`))
	})

	handler.HandleFunc("/bottest/getUpdates", func(w http.ResponseWriter, _ *http.Request) {
		if s.lastUpdateID < len(s.userMessages) {
			updates := make([]map[string]interface{}, 0)
			for i := s.lastUpdateID; i < len(s.userMessages); i++ {
				updates = append(updates, map[string]interface{}{
					"update_id": i + 1,
					"message":   s.userMessages[i],
				})
			}
			s.lastUpdateID = len(s.userMessages)

			response := map[string]interface{}{
				"ok":     true,
				"result": updates,
			}
			json.NewEncoder(w).Encode(response)
		} else {
			w.Write([]byte(`{"ok":true,"result":[]}`))
		}
	})

	handler.HandleFunc("/bottest/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Printf("error parsing form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		text := r.FormValue("text")
		s.lastBotMessage = text

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"ok": true,
			"result": {
				"message_id": 1
			}
		}`))
	})

	return httptest.NewServer(handler)
}

func (s *BotScrapperAgentSuite) SendUserMessage(text string) {
	s.userMessages = append(s.userMessages, map[string]interface{}{
		"message_id": len(s.userMessages) + 1,
		"from": map[string]interface{}{
			"id":       12345,
			"username": "testuser",
		},
		"chat": map[string]interface{}{
			"id": 12345,
		},
		"text": text,
	})

	time.Sleep(500 * time.Millisecond)
}

func (s *BotScrapperAgentSuite) TestValidMessageFlow() {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		future := time.Now().Add(1 * time.Hour).UTC()

		w.Header().Set("Last-Modified", future.Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	testURL := strings.Replace(testServer.URL, "127.0.0.1", "host.docker.internal", 1)

	req, err := http.NewRequest(http.MethodPost, s.scrapperURL+"/tg-chat/12345", nil)
	s.Require().NoError(err)
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	addLinkReq := map[string]interface{}{
		"link": testURL,
		"tags": []string{"go"},
	}
	body, err := json.Marshal(addLinkReq)
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodPost, s.scrapperURL+"/links", bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Tg-Chat-Id", "12345")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	waitTime := 30 * time.Second
	tickTime := 500 * time.Millisecond
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, testURL)
	}, waitTime, tickTime)
}

func (s *BotScrapperAgentSuite) TestInvalidMessageFormat() {
	brokers, err := s.kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: brokers,
		Topic:   s.rawTopic,
	})
	defer writer.Close()

	invalidData := []byte(`{invalid json`)
	err = writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte("1"),
		Value: invalidData,
	})
	s.Require().NoError(err)

	waitTime := 20 * time.Second
	tickTime := 500 * time.Millisecond
	dlqReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		Topic:       s.dlqRawTopic,
		GroupID:     "test-dlq-reader",
		StartOffset: kafkago.FirstOffset,
	})
	defer dlqReader.Close()

	s.Eventually(func() bool {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()

		msg, err := dlqReader.ReadMessage(ctx)
		if err != nil {
			return false
		}

		var dlqMsg map[string]interface{}
		if err := json.Unmarshal(msg.Value, &dlqMsg); err != nil {
			return false
		}

		return dlqMsg["error_type"] == "unmarshal"
	}, waitTime, tickTime)
}
