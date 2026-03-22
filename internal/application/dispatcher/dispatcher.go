package dispatcher

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type Command interface {
	Execute(msg *domain.Message) (*domain.Response, error)
	Name() string
	Description() string
}

type CommandDispatcher struct {
	commands       map[string]Command
	unknownCommand Command
}

func NewCommandDispatcher(unknownCommand Command) *CommandDispatcher {
	return &CommandDispatcher{commands: make(map[string]Command), unknownCommand: unknownCommand}
}

func (cd *CommandDispatcher) Register(cmd Command) {
	cd.commands[cmd.Name()] = cmd
}

func (cd *CommandDispatcher) Dispatch(msg *domain.Message) (*domain.Response, error) {
	if !strings.HasPrefix(msg.Text, "/") {
		slog.Debug("got a non-command message")
		return nil, errors.New("message is not a command")
	}

	parts := strings.Fields(msg.Text)
	cmdName := parts[0]

	handler, exists := cd.commands[cmdName]
	if exists {
		slog.Debug("found handler", "handler name", handler.Name(), "inputed command", cmdName)
		resp, err := handler.Execute(msg)
		if err != nil {
			return nil, fmt.Errorf("execute command %s: %w", cmdName, err)
		}
		return resp, nil
	}

	slog.Debug("handler not found", "inputed command", cmdName)
	resp, err := cd.unknownCommand.Execute(msg)
	if err != nil {
		return nil, fmt.Errorf("execute unknown command for %q: %w", msg.Text, err)
	}
	return resp, nil
}

func (cd *CommandDispatcher) GetCommands() []domain.BotCommand {
	botCommands := []domain.BotCommand{}
	for _, cmd := range cd.commands {
		botCommands = append(botCommands, domain.BotCommand{
			Name: cmd.Name(), Description: cmd.Description(),
		})
	}

	return botCommands
}
