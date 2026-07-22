package application

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

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
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

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

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestDispatch_UnknownCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

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

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse, resp)
}

func TestDispatch_NonCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

	msg := &domain.Message{
		Text:      "message",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	resp, err := dispatcher.Dispatch(msg)

	require.Error(t, err)
	assert.Nil(t, resp)
}
func TestDispatch_Conversation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().Name().Return("/track").AnyTimes()

	mockCmd2 := NewMockCommand(ctrl)
	mockCmd2.EXPECT().Name().Return("/cmd").AnyTimes()

	msg1 := &domain.Message{Text: "/track", ChatID: 1}
	msg2 := &domain.Message{Text: "next", ChatID: 1}

	resp1 := &domain.Response{Text: "step1", ChatID: 1}
	resp2 := &domain.Response{Text: "done", ChatID: 1}

	dispatcher.Register("/track", func() Command {
		return mockCmd
	})
	dispatcher.Register("/cmd", func() Command {
		return mockCmd2
	})

	// Проверка что выполнятся последовательно
	mockCmd.EXPECT().Execute(msg1).Return(resp1, false, nil)
	mockCmd.EXPECT().Execute(msg2).Return(resp2, true, nil)

	r1, _ := dispatcher.Dispatch(msg1)
	r2, _ := dispatcher.Dispatch(msg2)

	require.Equal(t, resp1, r1)
	require.Equal(t, resp2, r2)

	// /cancel
	mockCmd.EXPECT().Execute(msg1).Return(resp1, false, nil).Times(2)
	msgCancel := &domain.Message{Text: "/cancel", ChatID: 1}
	r3, _ := dispatcher.Dispatch(msg1)
	r4, _ := dispatcher.Dispatch(msgCancel)
	r5, _ := dispatcher.Dispatch(msg1)

	require.Equal(t, resp1, r3)
	require.Equal(t, &domain.Response{Text: "команда отменена", ChatID: 1}, r4)
	require.Equal(t, resp1, r5)

	// Запуск существующей команды во время conversation
	mockCmd.EXPECT().Execute(msg1).Return(resp1, false, nil).Times(2)
	msgAnotherCommand := &domain.Message{Text: "/cmd", ChatID: 1}
	mockCmd2.EXPECT().Execute(msgAnotherCommand).Return(&domain.Response{Text: "command complete", ChatID: 1}, true, nil)
	r6, _ := dispatcher.Dispatch(msg1)
	r7, _ := dispatcher.Dispatch(msgAnotherCommand)
	r8, _ := dispatcher.Dispatch(msg1)

	require.Equal(t, resp1, r6)
	require.Equal(t, &domain.Response{Text: "command complete", ChatID: 1}, r7)
	require.Equal(t, resp1, r8)
}

func TestGetCommands(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

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

func TestHandleMessage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

	msg := &domain.Message{
		Text:      "/start",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	expectedResp := &domain.Response{
		Text:   "Welcome!",
		ChatID: 12345,
	}

	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().Name().Return("/start")
	mockCmd.EXPECT().Execute(msg).Return(expectedResp, true, nil)
	dispatcher.Register("/start", func() Command { return mockCmd })

	bot.EXPECT().SendMessage(expectedResp).Return(nil).Times(1)

	dispatcher.HandleMessage(msg)
}

func TestHandleMessage_DispatchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

	msg := &domain.Message{
		Text:      "not a command",
		ChatID:    12345,
		Username:  "testuser",
		MessageID: 1,
	}

	bot.EXPECT().SendMessage(gomock.Any()).Do(func(resp *domain.Response) {
		assert.Equal(t, int64(12345), resp.ChatID)
		assert.Contains(t, resp.Text, "Произошла ошибка")
	}).Return(nil).Times(1)

	dispatcher.HandleMessage(msg)
}

func TestHandleUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	unknownCmd := NewMockCommand(ctrl)
	bot := NewMockBot(ctrl)
	dispatcher := NewCommandDispatcher(unknownCmd, bot, nil)

	chatIDs := []int64{1, 2, 3}
	desc := "test update"

	bot.EXPECT().SendMessage(&domain.Response{
		ChatID: 1,
		Text:   desc,
	}).Return(nil).Times(1)
	bot.EXPECT().SendMessage(&domain.Response{
		ChatID: 2,
		Text:   desc,
	}).Return(nil).Times(1)
	bot.EXPECT().SendMessage(&domain.Response{
		ChatID: 3,
		Text:   desc,
	}).Return(nil).Times(1)

	err := dispatcher.HandleUpdate(chatIDs, desc)
	require.NoError(t, err)
}
