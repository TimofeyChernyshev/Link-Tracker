package handlers

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler {
	return &StartHandler{}
}

func (sh *StartHandler) Execute(msg *domain.Message) (*domain.Response, error) {
	slog.Info("start requested", "chatID", msg.ChatID)

	text := "Добро пожаловать, " + msg.Username + "!\nИспользуйте /help, чтобы посмотреть доступные команды"

	return &domain.Response{
		Text:   text,
		ChatID: msg.ChatID,
	}, nil
}

func (sh *StartHandler) Name() string {
	return "/start"
}
