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
	minLinksOut   = 1
	maxLinksOut   = 50
	defaultOffset = 1
)

type ListHandler struct {
	timeout time.Duration

	step  int
	links []domain.Link
	tags  string
	limit int

	filteredLinks []string

	linkService LinkService
}

func NewListHandler(l LinkService, t time.Duration) *ListHandler {
	return &ListHandler{linkService: l, timeout: t}
}

func (lh *ListHandler) Execute(msg *domain.Message) (*domain.Response, bool, error) {
	slog.Info("list requested", "chatID", msg.ChatID)

	switch lh.step {
	case listStepAwaitingTags:
		lh.step = listStepAwaitingLimit

		context, cancel := context.WithTimeout(context.Background(), lh.timeout)
		defer cancel()

		links, err := lh.linkService.GetLinks(context, msg.ChatID)
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
	case listStepAwaitingLimit:
		lh.step = listStepAwaitingOffset

		lh.tags = msg.Text

		lh.filteredLinks = tagFilter(lh.links, lh.tags)

		if len(lh.filteredLinks) == 0 {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   "По указанным тегам ссылок не найдено",
			}, true, nil
		}

		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   "Сколько ссылок вывести за раз? (от 1 до 50):",
		}, false, nil
	case listStepAwaitingOffset:
		lh.step = listStepListing

		limit, err := strconv.Atoi(msg.Text)
		if err != nil || limit < minLinksOut {
			limit = minLinksOut
		}
		if limit > maxLinksOut {
			limit = maxLinksOut
		}
		lh.limit = limit

		return &domain.Response{
			ChatID: msg.ChatID,
			Text:   fmt.Sprintf("Начиная с какой позиции? (от 1 до %d):", len(lh.filteredLinks)),
		}, false, nil
	case listStepListing:
		offset, err := strconv.Atoi(msg.Text)
		if err != nil || offset < defaultOffset {
			offset = defaultOffset
		}
		startIndex := offset - 1

		if startIndex >= len(lh.filteredLinks) {
			return &domain.Response{
				ChatID: msg.ChatID,
				Text:   fmt.Sprintf("Позиция (%d) превышает количество ссылок (%d)", offset, len(lh.filteredLinks)),
			}, true, nil
		}

		end := startIndex + lh.limit
		if end > len(lh.filteredLinks) {
			end = len(lh.filteredLinks)
		}

		return &domain.Response{
			Text:   strings.Join(lh.filteredLinks[startIndex:end], "\n"),
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
