package handlers

import (
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
)

type HelpHandler struct{}

func NewHelpHandler() *HelpHandler {
	return &HelpHandler{}
}

func (hh *HelpHandler) Execute(msg *domain.Message) (*domain.Response, error) {
	slog.Info("help requested", "chatID", msg.ChatID)

	text := strings.Builder{}
	text.WriteString("Доступные команды:\n")
	text.WriteString("/start - начать работу\n")
	text.WriteString("/help - показать справку")

	return &domain.Response{
		Text:   text.String(),
		ChatID: msg.ChatID,
	}, nil
}

func (hh *HelpHandler) Name() string {
	return "/help"
}

func (hh *HelpHandler) Description() string {
	return "Выводит помощь"
}
