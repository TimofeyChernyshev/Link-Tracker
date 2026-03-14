package dispatcher

import (
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
}

func NewCommandDispatcher(unknownCommand Command) *CommandDispatcher {
	return &CommandDispatcher{
		factories:      make(map[string]func() Command),
		conversations:  sync.Map{},
		unknownCommand: unknownCommand,
	}
}

func (cd *CommandDispatcher) Register(name string, cmd func() Command) {
	cd.factories[name] = cmd
}

func (cd *CommandDispatcher) Dispatch(msg *domain.Message) (*domain.Response, error) {
	parts := strings.Fields(msg.Text)
	cmdName := parts[0]

	factory, exists := cd.factories[cmdName]
	isCommand := strings.HasPrefix(msg.Text, "/")
	if !exists || !isCommand {
		if convRaw, ok := cd.conversations.Load(msg.ChatID); ok {
			conv := convRaw.(Command)

			if cmdName == "/cancel" {
				cd.conversations.Delete(msg.ChatID)
				return &domain.Response{Text: "команда отменена", ChatID: msg.ChatID}, nil
			}

			resp, done, err := conv.Execute(msg)
			if done {
				cd.conversations.Delete(msg.ChatID)
			}

			return resp, err
		}

		if !isCommand {
			slog.Debug("got a non-command message")
			return nil, errors.New("message is not a command")
		}

		if !exists {
			slog.Debug("handler not found", "inputed command", cmdName)
			resp, _, _ := cd.unknownCommand.Execute(msg)
			return resp, fmt.Errorf("unknown command '%q'", msg.Text)
		}
	}
	cmd := factory()

	slog.Debug("found handler", "handler name", cmd.Name(), "inputed command", cmdName)

	resp, done, err := cmd.Execute(msg)
	if !done {
		cd.conversations.Store(msg.ChatID, cmd)
	}

	return resp, err
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
