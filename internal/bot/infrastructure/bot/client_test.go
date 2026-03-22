package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type BotClientSuite struct {
	suite.Suite
	client *Client
}

func (s *BotClientSuite) SetupTest() {

	api, cleanup := newTestBot(s.T())

	s.client = &Client{
		api:      api,
		stopChan: make(chan struct{}),
		jobs:     make(chan tgbotapi.Update, 10),
		outgoing: make(chan *domain.Response, 10),
		incoming: make(chan *domain.Message, 10),
	}

	s.T().Cleanup(cleanup)
}

func TestBotClientSuite(t *testing.T) {
	suite.Run(t, new(BotClientSuite))
}

func (s *BotClientSuite) TestReceive() {
	ctx := context.Background()

	received := make(chan *domain.Message, 1)
	go func() {
		msg, err := s.client.Receive(ctx)
		s.Require().NoError(err)
		received <- msg
	}()

	testMsg := &domain.Message{
		Text:      "/start",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}
	s.client.incoming <- testMsg

	select {
	case msg := <-received:
		s.Equal(testMsg.Text, msg.Text)
		s.Equal(testMsg.ChatID, msg.ChatID)
	case <-time.After(1 * time.Second):
		s.T().Fatal("Receive did not return")
	}
}

func (s *BotClientSuite) TestReceive_ContextCanceled() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg, err := s.client.Receive(ctx)
	s.Require().Error(err)
	s.Nil(msg)
	s.Equal(context.Canceled, err)
}

func (s *BotClientSuite) TestStop_WaitsSenders() {
	s.client.senderWg.Add(1)
	go s.client.sender()

	resp := &domain.Response{Text: "hello", ChatID: 1}
	s.client.SendMessage(resp)

	stopDone := make(chan struct{})
	var stopErr error
	go func() {
		defer close(stopDone)
		stopErr = s.client.Stop(context.Background())
	}()

	<-stopDone
	s.Require().NoError(stopErr)
	s.Eventually(func() bool {
		select {
		case _, ok := <-s.client.outgoing:
			return !ok
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func newTestBot(t *testing.T) (*tgbotapi.BotAPI, func()) {
	mux := http.NewServeMux()

	mux.HandleFunc("/botTESTTOKEN/getMe", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"ok": true,
			"result": {
				"id": 1,
				"is_bot": true,
				"first_name": "test",
				"username": "TestBot"
			}
		}`)
	})

	mux.HandleFunc("/botTESTTOKEN/sendMessage", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"ok": true,
			"result": {
				"message_id": 1
			}
		}`)
	})

	server := httptest.NewServer(mux)

	bot, err := tgbotapi.NewBotAPIWithClient("TESTTOKEN", server.URL+"/bot%s/%s", server.Client())
	require.NoError(t, err)

	return bot, server.Close
}
