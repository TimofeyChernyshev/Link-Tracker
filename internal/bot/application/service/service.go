package service

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Service struct {
	bot Bot
}

func New(bot Bot) *Service {
	return &Service{bot: bot}
}

func (s *Service) SendUpdates(chatIds []int64, desc string) {
	slog.Debug("got update", "description", desc, "chatIds", chatIds)

	for _, chatID := range chatIds {
		s.bot.SendMessage(&domain.Response{
			ChatID: chatID,
			Text:   desc,
		})
	}
}
