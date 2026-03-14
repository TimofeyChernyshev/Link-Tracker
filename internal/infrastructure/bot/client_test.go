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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type BotClientSuite struct {
	suite.Suite
	ctrl           *gomock.Controller
	mockDispatcher *MockCommandDispatcher
	client         *BotClient
}

func (s *BotClientSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockDispatcher = NewMockCommandDispatcher(s.ctrl)

	api, cleanup := newTestBot(s.T())

	s.client = &BotClient{
		api:        api,
		dispatcher: s.mockDispatcher,
		stopChan:   make(chan struct{}),
		jobs:       make(chan tgbotapi.Update, 10),
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

	mux.HandleFunc("/botTESTTOKEN/getMe", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("/botTESTTOKEN/sendMessage", func(w http.ResponseWriter, r *http.Request) {
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

	s.client.wg.Add(1)
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

	s.client.wg.Add(1)
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

	s.Error(err)
	s.Equal(context.DeadlineExceeded, err)

	close(handlerBlock)
	close(s.client.jobs)
}

// type BotClientSuite struct {
// 	suite.Suite
// 	ctrl           *gomock.Controller
// 	mockDispatcher *MockCommandDispatcher
// 	client         *BotClient
// 	server         *httptest.Server
// 	updatesChan    chan tgbotapi.Update
// 	started        chan struct{}
// 	botAPI         *tgbotapi.BotAPI
// }

// func (s *BotClientSuite) SetupTest() {
// 	s.ctrl = gomock.NewController(s.T())
// 	s.mockDispatcher = NewMockCommandDispatcher(s.ctrl)
// 	s.updatesChan = make(chan tgbotapi.Update, 10)
// 	s.started = make(chan struct{})

// 	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path == "/bot-test-token/getUpdates" {
// 			select {
// 			case update := <-s.updatesChan:
// 				resp := struct {
// 					Ok     bool              `json:"ok"`
// 					Result []tgbotapi.Update `json:"result"`
// 				}{
// 					Ok:     true,
// 					Result: []tgbotapi.Update{update},
// 				}
// 				json.NewEncoder(w).Encode(resp)
// 			default:
// 				json.NewEncoder(w).Encode(struct {
// 					Ok     bool              `json:"ok"`
// 					Result []tgbotapi.Update `json:"result"`
// 				}{Ok: true, Result: []tgbotapi.Update{}})
// 			}
// 			return
// 		}

// 		resp := struct {
// 			Ok     bool        `json:"ok"`
// 			Result interface{} `json:"result,omitempty"`
// 		}{
// 			Ok: true,
// 		}

// 		if r.URL.Path == "/test-token/getMe" {
// 			resp.Result = tgbotapi.User{
// 				ID:       123,
// 				UserName: "TestBot",
// 			}
// 		}

// 		json.NewEncoder(w).Encode(resp)
// 	}))

// 	var err error
// 	s.botAPI, err = tgbotapi.NewBotAPIWithClient(
// 		"test-token",
// 		s.server.URL,
// 		s.server.Client(),
// 	)
// 	s.Require().NoError(err)

// 	s.client = &BotClient{
// 		api:        s.botAPI,
// 		dispatcher: s.mockDispatcher,
// 		wg:         sync.WaitGroup{},
// 	}
// }

// func (s *BotClientSuite) TearDownTest() {
// 	s.ctrl.Finish()
// 	if s.server != nil {
// 		s.server.Close()
// 	}
// }

// func TestBotClientSuite(t *testing.T) {
// 	suite.Run(t, new(BotClientSuite))
// }

// func (s *BotClientSuite) startBot() (chan error, chan struct{}) {
// 	errChan := make(chan error, 1)
// 	ready := make(chan struct{})

// 	go func() {
// 		close(ready)
// 		err := s.client.Start()
// 		errChan <- err
// 		close(s.started)
// 	}()

// 	<-ready

// 	for s.client.stopChan == nil {
// 		time.Sleep(1 * time.Millisecond)
// 	}

// 	return errChan, ready
// }

// func (s *BotClientSuite) TestStop_WhileProcessingMessages() {
// 	handlerStarted := make(chan struct{})
// 	handlerRelease := make(chan struct{})
// 	handlerDone := make(chan struct{})

// 	s.mockDispatcher.EXPECT().
// 		Dispatch(gomock.Any()).
// 		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
// 			close(handlerStarted)
// 			<-handlerRelease
// 			close(handlerDone)
// 			return &domain.Response{Text: "ok", ChatID: 12345}, nil
// 		}).
// 		Times(1)

// 	errChan, _ := s.startBot()

// 	testUpdate := tgbotapi.Update{
// 		UpdateID: 1,
// 		Message: &tgbotapi.Message{
// 			Text:      "/start",
// 			Chat:      &tgbotapi.Chat{ID: 12345},
// 			From:      &tgbotapi.User{UserName: "testuser"},
// 			MessageID: 1,
// 		},
// 	}

// 	s.updatesChan <- testUpdate

// 	select {
// 	case <-handlerStarted:
// 	case <-time.After(100 * time.Millisecond):
// 		s.T().Fatal("handler did not start")
// 	}

// 	stopDone := make(chan struct{})
// 	var stopErr error
// 	go func() {
// 		defer close(stopDone)
// 		stopErr = s.client.Stop(context.Background())
// 	}()

// 	select {
// 	case <-stopDone:
// 		s.T().Fatal("Stop returned before handler finished")
// 	case <-time.After(50 * time.Millisecond):
// 	}

// 	close(handlerRelease)

// 	select {
// 	case <-handlerDone:
// 	case <-time.After(100 * time.Millisecond):
// 		s.T().Fatal("handler did not finish")
// 	}

// 	select {
// 	case <-stopDone:
// 		s.NoError(stopErr)
// 	case <-time.After(100 * time.Millisecond):
// 		s.T().Fatal("Stop did not return after handler finished")
// 	}

// 	select {
// 	case err := <-errChan:
// 		s.NoError(err)
// 	case <-time.After(100 * time.Millisecond):
// 		s.T().Fatal("Start did not return")
// 	}
// }

// // func (s *BotClientSuite) TestStop_Timeout() {
// // 	handlerStarted := make(chan struct{})
// // 	handlerPaused := make(chan struct{})

// // 	mockUser := tgbotapi.User{UserName: "TestBot"}
// // 	s.mockAPI.EXPECT().Self().Return(mockUser).Times(1)

// // 	updatesChan := make(chan tgbotapi.Update, 1)
// // 	s.mockAPI.EXPECT().GetUpdatesChan(gomock.Any()).Return(updatesChan).Times(1)
// // 	s.mockAPI.EXPECT().StopReceivingUpdates().Return().Times(1)

// // 	testUpdate := tgbotapi.Update{UpdateID: 1, Message: &tgbotapi.Message{
// // 		Text:      "/start",
// // 		Chat:      &tgbotapi.Chat{ID: 12345},
// // 		From:      &tgbotapi.User{UserName: "testuser"},
// // 		MessageID: 1,
// // 	}}
// // 	s.mockDispatcher.EXPECT().
// // 		Dispatch(gomock.Any()).
// // 		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
// // 			close(handlerStarted)
// // 			<-handlerPaused
// // 			return &domain.Response{Text: "ok", ChatID: 12345}, nil
// // 		}).
// // 		Times(1)

// // 	startDone := make(chan struct{})
// // 	go func() {
// // 		defer close(startDone)
// // 		err := s.client.Start()
// // 		s.Require().NoError(err)
// // 	}()

// // 	updatesChan <- testUpdate
// // 	<-handlerStarted

// // 	stopDone := make(chan struct{})
// // 	var stopErr error

// // 	go func() {
// // 		defer close(stopDone)
// // 		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
// // 		defer cancel()
// // 		stopErr = s.client.Stop(ctx)
// // 	}()

// // 	<-stopDone

// // 	s.Assert().Error(stopErr)
// // 	s.Assert().Equal(context.DeadlineExceeded, stopErr)

// // 	<-startDone
// // }
