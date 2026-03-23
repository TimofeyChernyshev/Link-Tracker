package handlers

import (
	"log/slog"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type HelpHandler struct{}

func NewHelpHandler() *HelpHandler {
	return &HelpHandler{}
}

func (hh *HelpHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("help requested", "chatID", msg.ChatID)

	text := strings.Builder{}
	text.WriteString("Доступные команды:\n")
	text.WriteString("/start - начать работу\n")
	text.WriteString("/help - показать справку\n")
	text.WriteString("/track - начать отслеживание ссылки. Можно указать один или несколько тегов, привязанных к ссылке\n")
	text.WriteString("/untrack - прекратить отслеживание ссылки\n")
	text.WriteString("/list - вывести список всех отслеживаемых ссылок. Можно указать тег для фильтрации по тегу")

	return &domain.Response{
		Text:   text.String(),
		ChatID: msg.ChatID,
	}, true, nil
}

func (hh *HelpHandler) Name() string {
	return "/help"
}

func (hh *HelpHandler) Description() string {
	return "Выводит помощь"
}
