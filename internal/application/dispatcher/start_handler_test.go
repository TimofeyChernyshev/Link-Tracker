package dispatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestStartHandler_Execute(t *testing.T) {
	handler := NewStartHandler()

	tests := []struct {
		name     string
		msg      *domain.Message
		expected struct {
			contains []string
			chatID   int64
		}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handler.Execute(tt.msg)

			assert.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expected.chatID, resp.ChatID)

			for _, substr := range tt.expected.contains {
				assert.Contains(t, resp.Text, substr)
			}
		})
	}
}
