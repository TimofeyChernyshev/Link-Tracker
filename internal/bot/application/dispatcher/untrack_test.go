package dispatcher

import (
	"context"
	"errors"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type UntrackHandlerSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	linkService *MockLinkService
	handler     *UntrackHandler
	msg         *domain.Message
}

func (s *UntrackHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.linkService = NewMockLinkService(s.ctrl)

	timeout := time.Second
	s.handler = NewUntrackHandler(s.linkService, timeout)
	s.msg = &domain.Message{
		Text:      "/untrack",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}
}

func (s *UntrackHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestUntrackHandlerSuite(t *testing.T) {
	suite.Run(t, new(UntrackHandlerSuite))
}

func (s *UntrackHandlerSuite) TestExecute_Success() {
	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.False(done)
	s.Require().NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Contains(resp.Text, "Введите ссылку")

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	s.linkService.EXPECT().
		RemoveLink(gomock.Any(), int64(12345), "https://github.com/test").
		Return(nil)

	resp, done, err = s.handler.Execute(linkMsg)
	s.Require().NoError(err)
	s.True(done)
	s.Require().NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Contains(resp.Text, "Ссылка больше не отслеживается")
}

func (s *UntrackHandlerSuite) TestExecute_Timeout() {
	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	s.linkService.EXPECT().
		RemoveLink(gomock.Any(), int64(12345), "https://github.com/test").
		DoAndReturn(func(ctx context.Context, _ int64, _ string) error {
			<-ctx.Done()
			return ctx.Err()
		})

	resp, done, err = s.handler.Execute(linkMsg)
	s.Require().Error(err)
	s.Nil(resp)
	s.ErrorContains(err, context.DeadlineExceeded.Error())
	s.True(done)
}

func (s *UntrackHandlerSuite) TestExecute_EmptyInput() {
	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	emptyMsg := &domain.Message{
		Text:      "",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	s.linkService.EXPECT().
		RemoveLink(gomock.Any(), int64(12345), "").
		Return(nil)

	resp, done, err = s.handler.Execute(emptyMsg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.True(done)
}

func (s *UntrackHandlerSuite) TestExecute_ValidationError() {
	expectedErr := errors.New("invalid url format")

	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "not-a-url",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	s.linkService.EXPECT().
		RemoveLink(gomock.Any(), int64(12345), "not-a-url").
		Return(expectedErr)

	resp, done, err = s.handler.Execute(linkMsg)
	s.Require().Error(err)
	s.ErrorContains(err, expectedErr.Error())
	s.True(done)
	s.Nil(resp)
}
