package dispatcher

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler {
	return &StartHandler{}
}

func (sh *StartHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("start requested", "chatID", msg.ChatID)

	text := "Добро пожаловать, " + msg.Username + "!\nИспользуйте /help, чтобы посмотреть доступные команды"

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
