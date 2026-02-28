package bot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type BotClientSuite struct {
	suite.Suite
	ctrl           *gomock.Controller
	mockDispatcher *MockCommandDispatcher
	client         *Client
}

func (s *BotClientSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockDispatcher = NewMockCommandDispatcher(s.ctrl)

	api, cleanup := newTestBot(s.T())

	s.client = &Client{
		api:        api,
		dispatcher: s.mockDispatcher,
		stopChan:   make(chan struct{}),
		jobs:       make(chan tgbotapi.Update, 10),
		outgoing:   make(chan *domain.Response, 10),
	}

	s.T().Cleanup(cleanup)
}

func (s *BotClientSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestBotClientSuite(t *testing.T) {
	suite.Run(t, new(BotClientSuite))
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

func (s *BotClientSuite) TestStop_WhileProcessingMessages() {
	handlerStarted := make(chan struct{})
	handlerRelease := make(chan struct{})

	s.client.workerWg.Add(1)
	go s.client.worker()

	update := tgbotapi.Update{
		UpdateID: 1,
		Message: &tgbotapi.Message{
			Text:      "/start",
			Chat:      &tgbotapi.Chat{ID: 12345},
			From:      &tgbotapi.User{UserName: "testuser"},
			MessageID: 1,
		},
	}

	s.mockDispatcher.EXPECT().
		Dispatch(gomock.Any()).
		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
			close(handlerStarted)
			<-handlerRelease
			return &domain.Response{Text: "ok", ChatID: 12345}, nil
		}).
		Times(1)

	s.client.jobs <- update

	<-handlerStarted

	stopDone := make(chan struct{})
	var stopErr error

	go func() {
		defer close(stopDone)
		stopErr = s.client.Stop(context.Background())
	}()

	select {
	case <-stopDone:
		s.T().Fatal("Stop returned before handler finished")
	default:
	}

	close(handlerRelease)
	close(s.client.jobs)

	<-stopDone
	s.NoError(stopErr)
}

func (s *BotClientSuite) TestStop_Timeout() {
	handlerStarted := make(chan struct{})
	handlerBlock := make(chan struct{})

	s.client.workerWg.Add(1)
	go s.client.worker()

	update := tgbotapi.Update{
		UpdateID: 1,
		Message: &tgbotapi.Message{
			Text: "/start",
			Chat: &tgbotapi.Chat{ID: 123},
			From: &tgbotapi.User{UserName: "test"},
		},
	}

	s.mockDispatcher.EXPECT().
		Dispatch(gomock.Any()).
		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
			close(handlerStarted)
			<-handlerBlock
			return &domain.Response{Text: "ok", ChatID: 123}, nil
		}).
		Times(1)

	s.client.jobs <- update

	<-handlerStarted

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := s.client.Stop(ctx)

	s.Require().Error(err)
	s.Require().ErrorContains(err, context.DeadlineExceeded.Error())

	close(handlerBlock)
	close(s.client.jobs)
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
