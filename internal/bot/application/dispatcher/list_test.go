package dispatcher

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type ListHandlerSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	linkService *MockLinkService
	handler     *ListHandler
	msg         *domain.Message
}

func (s *ListHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.linkService = NewMockLinkService(s.ctrl)

	timeout := time.Second
	s.handler = NewListHandler(s.linkService, timeout)
	s.msg = &domain.Message{
		Text:      "/list",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}
}

func (s *ListHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestListHandlerSuite(t *testing.T) {
	suite.Run(t, new(ListHandlerSuite))
}

func (s *ListHandlerSuite) TestExecute_SuccessWithFilter() {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"work", "urgent"}},
		{URL: "https://github.com/2", Tags: []string{"personal"}},
		{URL: "https://github.com/3", Tags: []string{"work", "archive"}},
	}

	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return(expectedLinks, nil).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)

	s.Require().NoError(err)
	s.False(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Contains(resp.Text, "Введите теги")

	filterMsg := &domain.Message{
		Text:      "work",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	resp, done, err = s.handler.Execute(filterMsg)

	s.Require().NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)

	expected := []string{
		"https://github.com/1",
		"https://github.com/3",
	}
	s.Equal(strings.Join(expected, "\n"), resp.Text)
}

func (s *ListHandlerSuite) TestExecute_FilterNoMatches() {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"work"}},
		{URL: "https://github.com/2", Tags: []string{"personal"}},
	}

	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return(expectedLinks, nil).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	filterMsg := &domain.Message{
		Text:      "nonexistent",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	resp, done, err = s.handler.Execute(filterMsg)

	s.Require().NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Equal("Список отслеживаемых ссылок пуст", resp.Text)
}

func (s *ListHandlerSuite) TestExecute_AllLinks() {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"work"}},
		{URL: "https://github.com/2", Tags: []string{"personal"}},
	}

	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return(expectedLinks, nil).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	filterMsg := &domain.Message{
		Text:      "-",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	resp, done, err = s.handler.Execute(filterMsg)

	s.Require().NoError(err)
	s.True(done)
	s.NotNil(resp)

	expected := []string{
		"https://github.com/1",
		"https://github.com/2",
	}
	s.Equal(strings.Join(expected, "\n"), resp.Text)
}

func (s *ListHandlerSuite) TestExecute_EmptyList() {
	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return([]domain.Link{}, nil).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)

	s.Require().NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Equal("Список отслеживаемых ссылок пуст", resp.Text)
}

func (s *ListHandlerSuite) TestExecute_EmptyListAfterFilter() {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"work"}},
		{URL: "https://github.com/2", Tags: []string{"personal"}},
	}

	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return(expectedLinks, nil)

	resp, done, err := s.handler.Execute(s.msg)
	s.Require().NoError(err)
	s.NotNil(resp)
	s.False(done)

	filterMsg := &domain.Message{
		Text:      "123",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	resp, done, err = s.handler.Execute(filterMsg)

	s.Require().NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Equal("Список отслеживаемых ссылок пуст", resp.Text)
}

func (s *ListHandlerSuite) TestExecute_ServiceError() {
	expectedErr := errors.New("service unavailable")

	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		Return(nil, expectedErr).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)

	s.Require().Error(err)
	s.Require().ErrorContains(err, expectedErr.Error())
	s.True(done)
	s.Nil(resp)
}

func (s *ListHandlerSuite) TestExecute_Timeout() {
	s.linkService.EXPECT().
		GetLinks(gomock.Any(), int64(12345)).
		DoAndReturn(func(ctx context.Context, _ int64) ([]domain.Link, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}).
		Times(1)

	resp, done, err := s.handler.Execute(s.msg)

	s.Require().Error(err)
	s.Require().ErrorContains(err, context.DeadlineExceeded.Error())
	s.True(done)
	s.Nil(resp)
}

func (s *ListHandlerSuite) TestTagFilter() {
	links := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"work", "urgent"}},
		{URL: "https://github.com/2", Tags: []string{"personal"}},
		{URL: "https://github.com/3", Tags: []string{"work", "archive"}},
		{URL: "https://github.com/4", Tags: []string{}},
	}

	tests := []struct {
		name     string
		tag      string
		expected []string
	}{
		{
			name:     "filter work",
			tag:      "work",
			expected: []string{"https://github.com/1", "https://github.com/3"},
		},
		{
			name:     "filter personal",
			tag:      "personal",
			expected: []string{"https://github.com/2"},
		},
		{
			name:     "unexistant tag",
			tag:      "123",
			expected: []string{},
		},
		{
			name:     "all links",
			tag:      "-",
			expected: []string{"https://github.com/1", "https://github.com/2", "https://github.com/3", "https://github.com/4"},
		},
		{
			name:     "empty tag",
			tag:      "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := tagFilter(links, tt.tag)
			s.Equal(tt.expected, result)
		})
	}
}
