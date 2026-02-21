package telegram

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestCommandDispatcher_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	cmd1 := NewMockCommand(ctrl)
	cmd1.EXPECT().Name().Return("/start").Times(1)

	cmd2 := NewMockCommand(ctrl)
	cmd2.EXPECT().Name().Return("/help").Times(1)

	dispatcher.Register(cmd1)
	dispatcher.Register(cmd2)

	assert.Len(t, dispatcher.commands, 2)
	assert.Equal(t, cmd1, dispatcher.commands["/start"])
	assert.Equal(t, cmd2, dispatcher.commands["/help"])
}

func TestCommandDispatcher_Dispatch_KnownCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().Name().Return("/test").Times(2)

	expectedResponse := &domain.Response{
		Text:   "test response",
		ChatID: 12345,
	}

	msg := &domain.Message{
		Text:      "/test arg1 arg2",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	mockCmd.EXPECT().Execute(msg).Return(expectedResponse, nil).Times(1)

	dispatcher.Register(mockCmd)

	resp, err := dispatcher.Dispatch(msg)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestCommandDispatcher_Dispatch_UnknownCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	expectedResponse := &domain.Response{
		Text:   "unknown command",
		ChatID: 12345,
	}

	msg := &domain.Message{
		Text:      "/unknown",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	unknownCmd.EXPECT().Execute(msg).Return(expectedResponse, nil).Times(1)

	resp, err := dispatcher.Dispatch(msg)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestCommandDispatcher_Dispatch_NonCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	msg := &domain.Message{
		Text:      "message",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	resp, err := dispatcher.Dispatch(msg)

	assert.NoError(t, err)
	assert.Nil(t, resp)
}
