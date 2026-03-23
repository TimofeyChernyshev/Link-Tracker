package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type UntrackHandler struct {
	timeout time.Duration

	linkService LinkService

	step int
}

func NewUntrackHandler(l LinkService, t time.Duration) *UntrackHandler {
	return &UntrackHandler{linkService: l, timeout: t}
}

func (uh *UntrackHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("untrack requested", "chatID", msg.ChatID, "step", uh.step)

	switch uh.step {
	case untrackStepAwaitingLink:
		uh.step = 1
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите ссылку, которую нужно перестать отслеживать",
		}, false, nil
	case untrackStepRemoving:
		context, cancel := context.WithTimeout(context.Background(), uh.timeout)
		defer cancel()

		err := uh.linkService.RemoveLink(context, msg.ChatID, msg.Text)
		if err != nil {
			slog.Error("untrack error", "error", err)
			return nil, true, fmt.Errorf("cannot remove link: %w", err)
		}

		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Ссылка больше не отслеживается",
		}, true, nil
	}

	return nil, true, nil
}

func (uh *UntrackHandler) Name() string {
	return "/untrack"
}

func (uh *UntrackHandler) Description() string {
	return " прекратить отслеживание ссылки"
}
