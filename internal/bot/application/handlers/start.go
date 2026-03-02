package handlers

import (
	"context"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type StartHandler struct {
	linkService LinkService
}

func NewStartHandler(l LinkService) *StartHandler {
	return &StartHandler{linkService: l}
}

func (sh *StartHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("start requested", "chatID", msg.ChatID)

	text := "Добро пожаловать, " + msg.Username + "!\nИспользуйте /help, чтобы посмотреть доступные команды"

	context, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := sh.linkService.RegisterChat(context, msg.ChatID)
	if err != nil {
		slog.Error("start error", "error", err)
		return nil, true, err
	}

	return &domain.Response{
		Text:   text,
		ChatID: msg.ChatID,
	}, true, nil
}

func (sh *StartHandler) Name() string {
	return "/start"
}

func (sh *StartHandler) Description() string {
	return "Выводит стартовое сообщение"
}
