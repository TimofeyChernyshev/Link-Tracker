package dispatcher

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type TrackHandler struct {
	linkService LinkService

	step int
	url  string
}

func NewTrackHandler(l LinkService) *TrackHandler {
	return &TrackHandler{linkService: l}
}

func (th *TrackHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("track requested", "chatID", msg.ChatID, "step", th.step)

	switch th.step {
	case 0:
		th.step = 1
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите ссылку для отслеживания",
		}, false, nil
	case 1:
		th.url = msg.Text
		th.step = 2
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите теги через пробел или '-', чтобы пропустить",
		}, false, nil
	case 2:
		tags := parseTags(msg.Text)
		slog.Debug("parsed tags", "tags", tags)

		context, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := th.linkService.AddLink(context, msg.ChatID, th.url, tags)
		if err != nil {
			slog.Error("track error", "error", err)
			return nil, true, err
		}

		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Ссылка добавлена",
		}, true, nil
	}

	return nil, true, nil
}

func (th *TrackHandler) Name() string {
	return "/track"
}

func (th *TrackHandler) Description() string {
	return "начать отслеживание ссылки. Можно указать один или несколько тегов, привязанных к ссылке"
}

func parseTags(tags string) []string {
	tags = strings.TrimSpace(tags)
	tags = strings.ToLower(tags)

	if tags == "-" {
		return []string{}
	}

	tagsSplitted := strings.Split(tags, ",")

	tagsSlice := []string{}
	for i := range tagsSplitted {
		tag := strings.TrimSpace(tagsSplitted[i])
		if tag != "" {
			tagsSlice = append(tagsSlice, tag)
		}
	}

	return tagsSlice
}
