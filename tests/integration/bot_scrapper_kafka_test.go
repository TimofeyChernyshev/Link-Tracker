package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type BotScrapperKafkaSuite struct {
	suite.Suite
	ctx          context.Context
	fakeTelegram *httptest.Server
	network      *testcontainers.DockerNetwork
	scrapper     testcontainers.Container
	bot          testcontainers.Container
	scrapperURL  string
	botURL       string
	lastUpdateID int
	userMessages []map[string]interface{}

	lastBotMessage string
	topic          string
	kafkaContainer *kafka.KafkaContainer

	postgresContainer testcontainers.Container
}

func TestBotScrapperKafkaSuite(t *testing.T) {
	suite.Run(t, new(BotScrapperKafkaSuite))
}

func (s *BotScrapperKafkaSuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	s.userMessages = make([]map[string]interface{}, 0)
	s.lastUpdateID = 0
	s.topic = "link-updates"

	s.fakeTelegram = s.StartFakeTelegram()

	u, _ := url.Parse(s.fakeTelegram.URL)
	telegramURL := strings.Replace(s.fakeTelegram.URL, u.Hostname(), "host.docker.internal", 1)

	net, err := network.New(s.ctx)
	s.Require().NoError(err)
	s.network = net

	postgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "linktracker",
		},
		Networks: []string{net.Name},
		NetworkAliases: map[string][]string{
			net.Name: {"postgres"},
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	postgresContainer, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: postgresReq,
		Started:          true,
	})
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

	brokers, err := kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(brokers)

	conn, err := kafkago.Dial("tcp", brokers[0])
	s.Require().NoError(err)
	defer conn.Close()

	err = conn.CreateTopics(kafkago.TopicConfig{
		Topic:             s.topic,
		NumPartitions:     3,
		ReplicationFactor: 1,
	})
	s.Require().NoError(err)

	scrapper, scrapperURL, err := StartScrapperWithKafka(s.ctx, net.Name, s.topic, []string{internalBroker})
	s.Require().NoError(err)
	s.scrapper = scrapper
	s.scrapperURL = scrapperURL

	bot, botURL, err := StartBotWithKafka(s.ctx, net.Name, telegramURL, s.topic, []string{internalBroker})
	s.Require().NoError(err)
	s.bot = bot
	s.botURL = botURL
}

func (s *BotScrapperKafkaSuite) TearDownSuite() {
	if s.bot != nil {
		s.bot.Terminate(s.ctx)
	}
	if s.scrapper != nil {
		s.scrapper.Terminate(s.ctx)
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

func (s *BotScrapperKafkaSuite) StartFakeTelegram() *httptest.Server {
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

func (s *BotScrapperKafkaSuite) SendUserMessage(text string) {
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

func (s *BotScrapperKafkaSuite) TestScrapperToKafkaToBot_LinkUpdate() {
	req, err := http.NewRequest(http.MethodPost, s.scrapperURL+"/tg-chat/22222", nil)
	s.Require().NoError(err)
	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	addLinkReq := map[string]interface{}{
		"link": "https://github.com/golang/go",
		"tags": []string{"go", "golang"},
	}
	body, err := json.Marshal(addLinkReq)
	s.Require().NoError(err)

	req, err = http.NewRequest(http.MethodPost, s.scrapperURL+"/links", bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Tg-Chat-Id", "22222")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Equal(http.StatusOK, resp.StatusCode)

	update := domain.LinkUpdate{
		ID:          1,
		URL:         "https://github.com/golang/go",
		Description: "update",
		TgChatIDs:   []int64{22222},
	}

	brokers, err := s.kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: brokers,
		Topic:   s.topic,
		Async:   false,
	})
	defer writer.Close()

	data, err := json.Marshal(update)
	s.Require().NoError(err)

	err = writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte("1"),
		Value: data,
	})
	s.Require().NoError(err)

	waitTime := 20 * time.Second
	tickTime := 500 * time.Millisecond
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "update")
	}, waitTime, tickTime)
}

func (s *BotScrapperKafkaSuite) TestFullCycle_UserAddsLink_ScrapperSendsUpdate() {
	waitTime := 10 * time.Second
	tickTime := 500 * time.Millisecond

	s.SendUserMessage("/start")
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "Добро пожаловать")
	}, waitTime, tickTime)

	s.SendUserMessage("/track")
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "Введите ссылку для отслеживания")
	}, waitTime, tickTime)

	s.SendUserMessage("https://github.com/golang/go")
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "Введите теги")
	}, waitTime, tickTime)

	s.SendUserMessage("golang")
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "Ссылка добавлена")
	}, waitTime, tickTime)

	req, err := http.NewRequest(http.MethodGet, s.scrapperURL+"/links", nil)
	s.Require().NoError(err)
	req.Header.Set("Tg-Chat-Id", "12345")

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var response struct {
		Links []struct {
			URL  string   `json:"url"`
			Tags []string `json:"tags"`
		} `json:"links"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	s.Require().NoError(err)

	s.Require().Len(response.Links, 1)
	s.Equal("https://github.com/golang/go", response.Links[0].URL)

	update := domain.LinkUpdate{
		ID:          1,
		URL:         "https://github.com/golang/go",
		Description: "update",
		TgChatIDs:   []int64{12345},
	}

	brokers, err := s.kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: brokers,
		Topic:   s.topic,
	})
	defer writer.Close()

	data, err := json.Marshal(update)
	s.Require().NoError(err)

	err = writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte(fmt.Sprintf("%d", update.ID)),
		Value: data,
	})
	s.Require().NoError(err)

	waitTime = 10 * time.Second
	tickTime = 500 * time.Millisecond
	s.Eventually(func() bool {
		return strings.Contains(s.lastBotMessage, "update")
	}, waitTime, tickTime, "Message not received within timeout")
}

func (s *BotScrapperKafkaSuite) TestScrapperNoSubscribers_NoNotification() {
	update := domain.LinkUpdate{
		ID:          999,
		URL:         "https://github.com/golang/go",
		Description: "This should not be sent",
		TgChatIDs:   []int64{99999},
	}

	brokers, err := s.kafkaContainer.Brokers(s.ctx)
	s.Require().NoError(err)

	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers: brokers,
		Topic:   s.topic,
	})
	defer writer.Close()

	data, err := json.Marshal(update)
	s.Require().NoError(err)

	err = writer.WriteMessages(s.ctx, kafkago.Message{
		Key:   []byte("999"),
		Value: data,
	})
	s.Require().NoError(err)

	lastMessageBefore := s.lastBotMessage

	s.Equal(lastMessageBefore, s.lastBotMessage)
}
