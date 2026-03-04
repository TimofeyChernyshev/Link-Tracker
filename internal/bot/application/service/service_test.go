package service

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestService_SendUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBot := NewMockBot(ctrl)
	svc := New(mockBot)

	chatIDs := []int64{1, 2, 3}
	desc := "hello"

	calls := 0

	mockBot.EXPECT().
		SendMessage(gomock.Any()).
		Times(len(chatIDs)).
		DoAndReturn(func(r *domain.Response) error {
			assert.Equal(t, chatIDs[calls], r.ChatID)
			assert.Equal(t, desc, r.Text)
			calls++
			return nil
		})

	svc.SendUpdates(chatIDs, desc)

	assert.Equal(t, len(chatIDs), calls)
}
