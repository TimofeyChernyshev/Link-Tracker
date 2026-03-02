package dispatcher

import (
	"context"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type UntrackHandler struct {
	linkService LinkService

	step int
}

func NewUntrackHandler(l LinkService) *UntrackHandler {
	return &UntrackHandler{linkService: l}
}

func (uh *UntrackHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("untrack requested", "chatID", msg.ChatID, "step", uh.step)

	switch uh.step {
	case 0:
		uh.step = 1
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите ссылку, которую нужно перестать отслеживать",
		}, false, nil
	case 1:
		context, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := uh.linkService.RemoveLink(context, msg.ChatID, msg.Text)
		if err != nil {
			slog.Error("untrack error", "error", err)
			return nil, true, err
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
