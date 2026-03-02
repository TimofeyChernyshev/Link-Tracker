package dispatcher

import (
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type TrackHandlerSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	linkService *MockLinkService
	handler     *TrackHandler
	msg         *domain.Message
}

func (s *TrackHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.linkService = NewMockLinkService(s.ctrl)
	s.handler = NewTrackHandler(s.linkService)
	s.msg = &domain.Message{
		Text:      "/track",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}
}

func (s *TrackHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestTrackHandlerSuite(t *testing.T) {
	suite.Run(t, new(TrackHandlerSuite))
}

func (s *TrackHandlerSuite) TestExecute_SuccessWithTags() {
	resp, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Contains(resp.Text, "Введите ссылку")

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}
	resp, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)
	s.NotNil(resp)
	s.Contains(resp.Text, "Введите теги")

	tagsMsg := &domain.Message{
		Text:      "work, urgent, bug",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{"work", "urgent", "bug"}).
		Return(nil)

	resp, done, err = s.handler.Execute(tagsMsg)

	s.NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Equal(int64(12345), resp.ChatID)
	s.Contains(resp.Text, "Ссылка добавлена")
}

func (s *TrackHandlerSuite) TestExecute_SuccessWithoutTags() {
	resp, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}
	resp, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)

	tagsMsg := &domain.Message{
		Text:      "-",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{}).
		Return(nil)

	resp, done, err = s.handler.Execute(tagsMsg)

	s.NoError(err)
	s.True(done)
	s.NotNil(resp)
	s.Contains(resp.Text, "Ссылка добавлена")
}

func (s *TrackHandlerSuite) TestExecute_AddLinkError() {
	expectedErr := errors.New("service unavailable")

	resp, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}
	resp, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)

	tagsMsg := &domain.Message{
		Text:      "work",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{"work"}).
		Return(expectedErr)

	resp, done, err = s.handler.Execute(tagsMsg)

	s.Error(err)
	s.Equal(expectedErr, err)
	s.True(done)
	s.Nil(resp)
}

func (s *TrackHandlerSuite) TestExecute_Timeout() {
	resp, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}
	resp, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)

	tagsMsg := &domain.Message{
		Text:      "work",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{"work"}).
		DoAndReturn(func(ctx context.Context, _ int64, _ string, _ []string) error {
			<-ctx.Done()
			return ctx.Err()
		})

	resp, done, err = s.handler.Execute(tagsMsg)

	s.Error(err)
	s.Equal(context.DeadlineExceeded, err)
	s.True(done)
	s.Nil(resp)
}

func (s *TrackHandlerSuite) TestExecute_EmptyTags() {
	resp, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}
	resp, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)

	tagsMsg := &domain.Message{
		Text:      "",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{}).
		Return(nil).
		Times(1)

	resp, done, err = s.handler.Execute(tagsMsg)

	s.NoError(err)
	s.True(done)
	s.NotNil(resp)
}

func (s *TrackHandlerSuite) TestExecute_TagsWithSpaces() {
	_, done, err := s.handler.Execute(s.msg)
	s.NoError(err)
	s.False(done)

	linkMsg := &domain.Message{
		Text:      "https://github.com/test",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 2,
	}

	_, done, err = s.handler.Execute(linkMsg)
	s.NoError(err)
	s.False(done)

	tagsMsg := &domain.Message{
		Text:      "  work  ,  urgent  ,  bug  ",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 3,
	}

	s.linkService.EXPECT().
		AddLink(gomock.Any(), int64(12345), "https://github.com/test", []string{"work", "urgent", "bug"}).
		Return(nil)

	_, done, err = s.handler.Execute(tagsMsg)

	s.NoError(err)
	s.True(done)
}

func (s *TrackHandlerSuite) TestParseTags() {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "default tags",
			input:    "work,urgent,bug",
			expected: []string{"work", "urgent", "bug"},
		},
		{
			name:     "tags with spaces",
			input:    "  work  ,  urgent  ,  bug , tag 123 ",
			expected: []string{"work", "urgent", "bug", "tag 123"},
		},
		{
			name:     "skip tags",
			input:    "-",
			expected: []string{},
		},
		{
			name:     "empty tag",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: []string{},
		},
		{
			name:     "one tag",
			input:    "work",
			expected: []string{"work"},
		},
		{
			name:     "tag with empty element",
			input:    "work,,urgent",
			expected: []string{"work", "urgent"},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := parseTags(tt.input)
			s.Equal(tt.expected, result)
		})
	}
}
