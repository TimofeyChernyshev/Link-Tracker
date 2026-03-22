package handlers

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestStartHandler_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	linkService := NewMockLinkService(ctrl)

	timeout := time.Second
	handler := NewStartHandler(linkService, timeout)

	tests := []struct {
		name     string
		msg      *domain.Message
		expected struct {
			contains []string
			chatID   int64
		}
		linkServiceErr error
	}{
		{
			name: "start request",
			msg: &domain.Message{
				Text:      "/start",
				ChatID:    12345,
				Username:  "user",
				MessageID: 1,
			},
			expected: struct {
				contains []string
				chatID   int64
			}{
				contains: []string{
					"Добро пожаловать",
					"/help",
					"user",
				},
				chatID: 12345,
			},
		},
		{
			name: "start request with argument",
			msg: &domain.Message{
				Text:      "/start 123",
				ChatID:    12345,
				Username:  "user",
				MessageID: 1,
			},
			expected: struct {
				contains []string
				chatID   int64
			}{
				contains: []string{
					"Добро пожаловать",
					"/help",
					"user",
				},
				chatID: 12345,
			},
		},
		{
			name: "link service return err",
			msg: &domain.Message{
				Text:      "/start 123",
				ChatID:    12345,
				Username:  "user",
				MessageID: 1,
			},
			expected: struct {
				contains []string
				chatID   int64
			}{
				contains: []string{
					"Добро пожаловать",
					"/help",
					"user",
				},
				chatID: 12345,
			},
			linkServiceErr: errors.New("some error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linkService.EXPECT().RegisterChat(gomock.Any(), tt.msg.ChatID).Return(tt.linkServiceErr).Times(1)

			resp, done, err := handler.Execute(tt.msg)
			assert.True(t, done)

			require.ErrorIs(t, err, tt.linkServiceErr)
			if err != nil {
				require.Nil(t, resp)
			} else {
				require.NotNil(t, resp)
				assert.Equal(t, tt.expected.chatID, resp.ChatID)
				for _, substr := range tt.expected.contains {
					assert.Contains(t, resp.Text, substr)
				}
			}
		})
	}
}
