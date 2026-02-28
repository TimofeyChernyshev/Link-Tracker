package dispatcher

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	cmd1 := NewMockCommand(ctrl)
	cmd2 := NewMockCommand(ctrl)

	dispatcher.Register("/start", func() Command { return cmd1 })
	dispatcher.Register("/help", func() Command { return cmd2 })

	assert.Len(t, dispatcher.factories, 2)
	assert.Equal(t, cmd1, dispatcher.factories["/start"]())
	assert.Equal(t, cmd2, dispatcher.factories["/help"]())
}

func TestDispatch_KnownCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	mockCmd := NewMockCommand(ctrl)

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

	mockCmd.EXPECT().Name().Return("/test").AnyTimes()
	mockCmd.EXPECT().Execute(msg).Return(expectedResponse, true, nil)

	dispatcher.Register("/test", func() Command {
		return mockCmd
	})

	resp, err := dispatcher.Dispatch(msg)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestDispatch_UnknownCommand(t *testing.T) {
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

	unknownCmd.EXPECT().Execute(msg).Return(expectedResponse, true, nil).Times(1)

	resp, err := dispatcher.Dispatch(msg)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestDispatch_NonCommand(t *testing.T) {
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

func TestDispatch_Conversation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().Name().Return("/track").AnyTimes()

	msg1 := &domain.Message{Text: "/track", ChatID: 1}
	msg2 := &domain.Message{Text: "next", ChatID: 1}

	resp1 := &domain.Response{Text: "step1"}
	resp2 := &domain.Response{Text: "done"}

	gomock.InOrder(
		mockCmd.EXPECT().Execute(msg1).Return(resp1, false, nil),
		mockCmd.EXPECT().Execute(msg2).Return(resp2, true, nil),
	)

	d := NewCommandDispatcher(nil)

	d.Register("/track", func() Command {
		return mockCmd
	})

	r1, _ := d.Dispatch(msg1)
	r2, _ := d.Dispatch(msg2)

	assert.Equal(t, resp1, r1)
	assert.Equal(t, resp2, r2)
}

func TestGetCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd)

	cmd1 := NewMockCommand(ctrl)
	cmd1.EXPECT().Name().Return("/test1")
	cmd1.EXPECT().Description().Return("test1 description")

	cmd2 := NewMockCommand(ctrl)
	cmd2.EXPECT().Name().Return("/test2")
	cmd2.EXPECT().Description().Return("test2 description")

	dispatcher.Register("/test1", func() Command { return cmd1 })
	dispatcher.Register("/test2", func() Command { return cmd2 })

	expected := []domain.BotCommand{
		{Name: "/test1", Description: "test1 description"},
		{Name: "/test2", Description: "test2 description"},
	}

	got := dispatcher.GetCommands()

	require.ElementsMatch(t, expected, got)
}
