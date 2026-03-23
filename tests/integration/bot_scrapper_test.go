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

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
)

type BotScrapperSuite struct {
	suite.Suite
	ctx            context.Context
	fakeTelegram   *httptest.Server
	network        *testcontainers.DockerNetwork
	scrapper       testcontainers.Container
	bot            testcontainers.Container
	scrapperURL    string
	botURL         string
	lastUpdateID   int
	userMessages   []map[string]interface{}
	lastBotMessage string
}

func TestBotScrapperSuite(t *testing.T) {
	suite.Run(t, new(BotScrapperSuite))
}

func (s *BotScrapperSuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	s.userMessages = make([]map[string]interface{}, 0)
	s.lastUpdateID = 0

	s.fakeTelegram = s.StartFakeTelegram()

	u, _ := url.Parse(s.fakeTelegram.URL)
	telegramURL := strings.Replace(s.fakeTelegram.URL, u.Hostname(), "host.docker.internal", 1)

	network, err := network.New(s.ctx)
	s.Require().NoError(err)
	s.network = network

	scrapper, scrapperURL, err := StartScrapper(s.ctx, network.Name)
	s.Require().NoError(err)
	s.scrapper = scrapper
	s.scrapperURL = scrapperURL

	bot, botURL, err := StartBot(s.ctx, network.Name, telegramURL)
	s.Require().NoError(err)
	s.bot = bot
	s.botURL = botURL
}

func (s *BotScrapperSuite) TearDownSuite() {
	if s.bot != nil {
		s.bot.Terminate(s.ctx)
	}
	if s.scrapper != nil {
		s.scrapper.Terminate(s.ctx)
	}
	if s.network != nil {
		s.network.Remove(s.ctx)
	}
	if s.fakeTelegram != nil {
		s.fakeTelegram.Close()
	}
}

func (s *BotScrapperSuite) StartFakeTelegram() *httptest.Server {
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

func (s *BotScrapperSuite) SendUserMessage(text string) {
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

func (s *BotScrapperSuite) TestBotScrapperIntegration() {
	s.Run("bot register chat in scrapper", func() {
		s.SendUserMessage("/start")

		req, _ := http.NewRequest(http.MethodGet, s.scrapperURL+"/links", nil)
		req.Header.Set("Tg-Chat-Id", "12345")

		resp, err := http.DefaultClient.Do(req)
		s.Require().NoError(err)
		defer resp.Body.Close()

		s.Equal(http.StatusOK, resp.StatusCode)
		s.Contains(s.lastBotMessage, "Добро пожаловать")
	})

	// Отправка /track в зарегестрированный чат
	s.SendUserMessage("/track")
	s.SendUserMessage("https://github.com/golang/go")
	s.SendUserMessage("1, 2, 3")

	s.Run("list links", func() {
		// Проверка, что ссылка сохранена правильно в Scrapper
		req, _ := http.NewRequest(http.MethodGet, s.scrapperURL+"/links", nil)
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

		s.Len(response.Links, 1)
		s.Equal("https://github.com/golang/go", response.Links[0].URL)
		s.ElementsMatch([]string{"3", "1", "2"}, response.Links[0].Tags)

		// Проверка отправки команды в бота для получения ссылки из Scrapper
		s.SendUserMessage("/list")
		s.SendUserMessage("-")

		s.Equal("https://github.com/golang/go", s.lastBotMessage)
	})

	s.Run("Untrack link", func() {
		s.SendUserMessage("/untrack")
		s.SendUserMessage("https://github.com/golang/go")
		s.Contains(s.lastBotMessage, "не отслеживается")

		req, _ := http.NewRequest(http.MethodGet, s.scrapperURL+"/links", nil)
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

		s.Empty(response.Links)
	})

	s.Run("Delete chat", func() {
		req, _ := http.NewRequest(http.MethodDelete, s.scrapperURL+"/tg-chat/12345", nil)

		resp, err := http.DefaultClient.Do(req)
		s.Require().NoError(err)
		defer resp.Body.Close()

		s.Equal(http.StatusOK, resp.StatusCode)

		req, _ = http.NewRequest(http.MethodGet, s.scrapperURL+"/links", nil)
		req.Header.Set("Tg-Chat-Id", "12345")

		resp, err = http.DefaultClient.Do(req)
		s.Require().NoError(err)
		defer resp.Body.Close()

		s.Equal(http.StatusNotFound, resp.StatusCode)
	})
}

type LinkUpdate struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	TgChatIDs   []int64 `json:"tgChatIds"`
}

func (s *BotScrapperSuite) TestScrapperNotifiesBot() {
	s.SendUserMessage("/start")

	s.SendUserMessage("/track")
	s.SendUserMessage("https://github.com/golang/go")
	s.SendUserMessage("go")

	update := LinkUpdate{
		ID:          1,
		URL:         "https://github.com/golang/go",
		Description: "New commit in repository",
		TgChatIDs:   []int64{12345},
	}

	body, err := json.Marshal(update)
	s.Require().NoError(err)

	resp, err := http.Post(s.botURL+"/updates", "application/json", bytes.NewReader(body))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	time.Sleep(1 * time.Second)

	s.Contains(s.lastBotMessage, "New commit in repository")
}
