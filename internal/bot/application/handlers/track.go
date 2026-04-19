package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type TrackHandler struct {
	timeoutSaving    time.Duration
	timeoutCheckLink time.Duration

	linkService LinkService

	step int
	url  string
}

func NewTrackHandler(l LinkService, timeoutSaving, timeoutCheckLink time.Duration) *TrackHandler {
	return &TrackHandler{linkService: l, timeoutSaving: timeoutSaving, timeoutCheckLink: timeoutCheckLink}
}

func (th *TrackHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("track requested", "chatID", msg.ChatID, "step", th.step)

	switch th.step {
	case trackStepAwaitingLink:
		th.step = 1
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите ссылку для отслеживания",
		}, false, nil
	case trackStepAwaitingTags:
		_, err := url.ParseRequestURI(msg.Text)
		if err != nil {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "Некорректный формат ссылки",
			}, true, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), th.timeoutCheckLink)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, msg.Text, nil)
		if err != nil {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "Некорректный формат ссылки",
			}, true, nil
		}

		client := &http.Client{Timeout: th.timeoutCheckLink}
		resp, err := client.Do(req)
		if err != nil {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "Ссылка недоступна",
			}, true, nil
		}
		defer func() {
			err = resp.Body.Close()
			if err != nil {
				slog.Error("failed to close response body", "error", err)
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   fmt.Sprintf("Ссылка не отвечает (статус %d)", resp.StatusCode),
			}, true, nil
		}

		th.url = msg.Text
		th.step = 2
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите теги через пробел или '-', чтобы пропустить",
		}, false, nil
	case trackStepSaving:
		tags := parseTags(msg.Text)
		slog.Debug("parsed tags", "tags", tags)

		context, cancel := context.WithTimeout(context.Background(), th.timeoutSaving)
		defer cancel()

		err := th.linkService.AddLink(context, msg.ChatID, th.url, tags)
		if err != nil {
			slog.Error("track error", "error", err)
			return nil, true, fmt.Errorf("cannot add link: %w", err)
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
