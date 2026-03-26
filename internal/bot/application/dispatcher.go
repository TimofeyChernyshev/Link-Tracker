package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type CommandDispatcher struct {
	factories      map[string]func() Command
	unknownCommand Command

	conversations sync.Map // chatID -> active dialog

	bot Bot
}

func NewCommandDispatcher(unknownCommand Command, bot Bot) *CommandDispatcher {
	return &CommandDispatcher{
		factories:      make(map[string]func() Command),
		conversations:  sync.Map{},
		unknownCommand: unknownCommand,
		bot:            bot,
	}
}

func (cd *CommandDispatcher) Run(ctx context.Context) {
	for {
		msg, err := cd.bot.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			slog.Error("receive error", "error", err)
			continue
		}

		cd.HandleMessage(msg)
	}
}

func (cd *CommandDispatcher) HandleMessage(msg *domain.Message) {
	resp, err := cd.Dispatch(msg)
	if err != nil {
		slog.Error("dispatch error", "error", err)

		cd.bot.SendMessage(&domain.Response{
			ChatID: msg.ChatID,
			Text:   "Произошла ошибка: " + err.Error(),
		})
		return
	}

	if resp != nil {
		cd.bot.SendMessage(resp)
	}
}

func (cd *CommandDispatcher) HandleUpdate(chatIDs []int64, desc string) {
	slog.Debug("got update", "description", desc, "chatIds", chatIDs)

	for _, chatID := range chatIDs {
		cd.bot.SendMessage(&domain.Response{
			ChatID: chatID,
			Text:   desc,
		})
	}
}

func (cd *CommandDispatcher) Register(name string, cmd func() Command) {
	cd.factories[name] = cmd
}

func (cd *CommandDispatcher) GetCommands() []domain.BotCommand {
	botCommands := []domain.BotCommand{}
	for _, factory := range cd.factories {
		cmd := factory()
		botCommands = append(botCommands, domain.BotCommand{
			Name: cmd.Name(), Description: cmd.Description(),
		})
	}

	return botCommands
}

func (cd *CommandDispatcher) Dispatch(msg *domain.Message) (*domain.Response, error) {
	isCommand := strings.HasPrefix(msg.Text, "/")

	convRaw, hasConversation := cd.conversations.Load(msg.ChatID)

	if hasConversation {
		cmd, ok := convRaw.(Command)
		if !ok {
			return nil, errors.New("cannot convert conversation to command")
		}
		return cd.handleConversation(cmd, msg)
	}

	return cd.handleNewCommand(msg, isCommand)
}

func (cd *CommandDispatcher) handleConversation(conv Command, msg *domain.Message) (*domain.Response, error) {
	if msg.Text == "/cancel" {
		cd.conversations.Delete(msg.ChatID)
		return &domain.Response{
			Text:   "команда отменена",
			ChatID: msg.ChatID,
		}, nil
	}

	// Если введена другая команда во время выполнения текущей, то выполнение прерывается
	if strings.HasPrefix(msg.Text, "/") {
		parts := strings.Fields(msg.Text)
		cmdName := parts[0]

		if _, exists := cd.factories[cmdName]; exists {
			cd.conversations.Delete(msg.ChatID)
			return cd.handleNewCommand(msg, true)
		}
	}

	resp, done, err := conv.Execute(msg)
	if done {
		cd.conversations.Delete(msg.ChatID)
	}
	if err != nil {
		return nil, fmt.Errorf("handling command error: %w", err)
	}

	return resp, nil
}

func (cd *CommandDispatcher) handleNewCommand(msg *domain.Message, isCommand bool) (*domain.Response, error) {
	if !isCommand {
		slog.Debug("got a non-command message")
		return nil, errors.New("message is not a command")
	}

	parts := strings.Fields(msg.Text)
	cmdName := parts[0]

	factory, exists := cd.factories[cmdName]
	if !exists {
		slog.Debug("handler not found", "command", cmdName)
		resp, _, _ := cd.unknownCommand.Execute(msg)
		return resp, nil
	}

	cmd := factory()
	slog.Debug("found handler", "name", cmd.Name(), "command", cmdName)

	resp, done, err := cmd.Execute(msg)
	if !done {
		cd.conversations.Store(msg.ChatID, cmd)
	}
	if err != nil {
		return nil, fmt.Errorf("handling command error: %w", err)
	}

	return resp, nil
}
