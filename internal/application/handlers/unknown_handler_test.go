package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestUnknownHandler_Execute(t *testing.T) {
	handler := NewUnknownHandler()

	tests := []struct {
		name     string
		msg      *domain.Message
		expected struct {
			contains []string
			chatID   int64
		}
	}{
		{
			name: "unknown command",
			msg: &domain.Message{
				Text:      "/cmd",
				ChatID:    12345,
				Username:  "user",
				MessageID: 1,
			},
			expected: struct {
				contains []string
				chatID   int64
			}{
				contains: []string{
					"неизвестная команда",
					"/help",
				},
				chatID: 12345,
			},
		},
		{
			name: "unknown command with argument",
			msg: &domain.Message{
				Text:      "/cmd 123",
				ChatID:    12345,
				Username:  "user",
				MessageID: 1,
			},
			expected: struct {
				contains []string
				chatID   int64
			}{
				contains: []string{
					"неизвестная команда",
					"/help",
				},
				chatID: 12345,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, done, err := handler.Execute(tt.msg)
			assert.Equal(t, true, done)

			assert.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, tt.expected.chatID, resp.ChatID)

			for _, substr := range tt.expected.contains {
				assert.Contains(t, resp.Text, substr)
			}
		})
	}
}
