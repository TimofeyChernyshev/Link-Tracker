package handlers

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type ListHandler struct {
	step  int
	links []domain.Link

	linkService LinkService
}

func NewListHandler(l LinkService) *ListHandler {
	return &ListHandler{linkService: l}
}

func (lh *ListHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("list requested", "chatID", msg.ChatID)

	switch lh.step {
	case 0:
		lh.step = 1

		context, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		links, err := lh.linkService.GetLinks(context, msg.ChatID)
		if err != nil {
			slog.Error("list error", "error", err)
			return nil, true, err
		}

		if len(links) == 0 {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "Список отслеживаемых ссылок пуст",
			}, true, nil
		}

		lh.links = links
		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Введите теги по которым нужно провести фильтрацию или '-', чтобы вывести все ссылки",
		}, false, nil
	case 1:
		filteredLinks := tagFilter(lh.links, msg.Text)

		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   strings.Join(filteredLinks, "\n"),
		}, true, nil
	}

	return nil, true, nil
}

func tagFilter(links []domain.Link, tag string) []string {
	ans := []string{}

	for _, l := range links {
		if tag == "-" || slices.Contains(l.Tags, tag) {
			ans = append(ans, l.URL)
		}
	}

	return ans
}

func (lh *ListHandler) Name() string {
	return "/list"
}

func (lh *ListHandler) Description() string {
	return "вывести список всех отслеживаемых ссылок. Можно указать тег для фильтрации по тегу"
}
