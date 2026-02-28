package bot

import (
	"context"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type BotClientSuite struct {
	suite.Suite
	ctrl           *gomock.Controller
	mockAPI        *MockTelegramAPI
	mockDispatcher *MockCommandDispatcher
	client         *BotClient
	updatesChan    chan tgbotapi.Update
	started        chan struct{}
}

func (s *BotClientSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockAPI = NewMockTelegramAPI(s.ctrl)
	s.mockDispatcher = NewMockCommandDispatcher(s.ctrl)
	s.updatesChan = make(chan tgbotapi.Update, 10)
	s.started = make(chan struct{})

	s.client = &BotClient{
		api:        s.mockAPI,
		dispatcher: s.mockDispatcher,
		wg:         sync.WaitGroup{},
	}
}

func (s *BotClientSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestBotClientSuite(t *testing.T) {
	suite.Run(t, new(BotClientSuite))
}

func (s *BotClientSuite) TestStop_WhileProcessingMessages() {
	handlerStarted := make(chan struct{})
	handlerRelease := make(chan struct{})

	mockUser := tgbotapi.User{UserName: "TestBot"}
	s.mockAPI.EXPECT().Self().Return(mockUser).Times(1)

	updatesChan := make(chan tgbotapi.Update, 1)
	s.mockAPI.EXPECT().GetUpdatesChan(gomock.Any()).Return(updatesChan).Times(1)
	s.mockAPI.EXPECT().StopReceivingUpdates().Return().Times(1)
	s.mockAPI.EXPECT().Send(gomock.Any()).Return(tgbotapi.Message{}, nil).Times(1)

	testUpdate := tgbotapi.Update{UpdateID: 1, Message: &tgbotapi.Message{
		Text:      "/start",
		Chat:      &tgbotapi.Chat{ID: 12345},
		From:      &tgbotapi.User{UserName: "testuser"},
		MessageID: 1,
	}}
	s.mockDispatcher.EXPECT().
		Dispatch(gomock.Any()).
		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
			close(handlerStarted)
			<-handlerRelease
			return &domain.Response{Text: "ok", ChatID: 12345}, nil
		}).
		Times(1)

	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		err := s.client.Start()
		s.Require().NoError(err)
	}()

	updatesChan <- testUpdate
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

	<-stopDone
	s.NoError(stopErr)

	<-startDone
}

func (s *BotClientSuite) TestStop_Timeout() {
	handlerStarted := make(chan struct{})
	handlerPaused := make(chan struct{})

	mockUser := tgbotapi.User{UserName: "TestBot"}
	s.mockAPI.EXPECT().Self().Return(mockUser).Times(1)

	updatesChan := make(chan tgbotapi.Update, 1)
	s.mockAPI.EXPECT().GetUpdatesChan(gomock.Any()).Return(updatesChan).Times(1)
	s.mockAPI.EXPECT().StopReceivingUpdates().Return().Times(1)

	testUpdate := tgbotapi.Update{UpdateID: 1, Message: &tgbotapi.Message{
		Text:      "/start",
		Chat:      &tgbotapi.Chat{ID: 12345},
		From:      &tgbotapi.User{UserName: "testuser"},
		MessageID: 1,
	}}
	s.mockDispatcher.EXPECT().
		Dispatch(gomock.Any()).
		DoAndReturn(func(*domain.Message) (*domain.Response, error) {
			close(handlerStarted)
			<-handlerPaused
			return &domain.Response{Text: "ok", ChatID: 12345}, nil
		}).
		Times(1)

	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		err := s.client.Start()
		s.Require().NoError(err)
	}()

	updatesChan <- testUpdate
	<-handlerStarted

	stopDone := make(chan struct{})
	var stopErr error

	go func() {
		defer close(stopDone)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		stopErr = s.client.Stop(ctx)
	}()

	<-stopDone

	s.Assert().Error(stopErr)
	s.Assert().Equal(context.DeadlineExceeded, stopErr)

	<-startDone
}
