package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const (
	minPage      = 1
	linksPerPage = 50
)

type ListHandler struct {
	timeout time.Duration

	step  int
	links []domain.Link

	linkService LinkService
}

func NewListHandler(l LinkService, t time.Duration) *ListHandler {
	return &ListHandler{linkService: l, timeout: t}
}

func (lh *ListHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("list requested", "chatID", msg.ChatID, "command", msg.Text)

	parts := strings.Fields(msg.Text)
	page := minPage
	if len(parts) > 1 {
		if p, err := strconv.Atoi(parts[1]); err == nil && p >= minPage {
			page = p
		}
	}

	switch lh.step {
	case listStepAwaitingTags:
		lh.step = listStepListing

		context, cancel := context.WithTimeout(context.Background(), lh.timeout)
		defer cancel()

		links, err := lh.linkService.GetLinks(context, msg.ChatID, linksPerPage, (page-1)*linksPerPage)
		if err != nil {
			slog.Error("list error", "error", err)
			return nil, true, fmt.Errorf("cannot get links: %w", err)
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
	case listStepListing:
		filteredLinks := tagFilter(lh.links, msg.Text)
		if len(filteredLinks) == 0 {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "По указанным тегам ссылок не найдено",
			}, true, nil
		}

		return &domain.Response{
			Text:   strings.Join(filteredLinks, "\n"),
			ChatID: msg.ChatID,
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
