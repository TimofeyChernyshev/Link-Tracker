package handlers

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type UnknownHandler struct{}

func NewUnknownHandler() *UnknownHandler {
	return &UnknownHandler{}
}

func (uh *UnknownHandler) Execute(msg *domain.Message) (*domain.Response, error) {
	slog.Warn("unknown command", "chatID", msg.ChatID)

	text := "Введена неизвестная команда\nИспользуйте /help, чтобы посмотреть доступные команды"

	return &domain.Response{
		Text:   text,
		ChatID: msg.ChatID,
	}, nil
}

func (uh *UnknownHandler) Name() string {
	return ""
}
