package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

var (
	errChatInstRegistered = errors.New("chat isn't registered")

	defaultOffset = 0
)

type Service struct {
	client       Client
	notifier     Notifier
	cache        Cache
	storage      Storage
	batchSize    int
	workerCount  int
	defaultLimit int
}

func NewLinkService(c Client, n Notifier, s Storage, cache Cache, bs, wk, dl int) *Service {
	return &Service{client: c, notifier: n, storage: s, cache: cache, batchSize: bs, workerCount: wk, defaultLimit: dl}
}

func (s *Service) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.Link, error) {
	slog.Debug("adding link", "chatID", chatID, "url", url, "tags", tags)

	exists, err := s.storage.ChatExists(ctx, chatID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("check chat existence: %w", err)
	}
	if !exists {
		slog.Warn("cannot add link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	subscribed, err := s.storage.IsSubscribed(ctx, chatID, url)
	if err != nil {
		return domain.Link{}, fmt.Errorf("check link subscription: %w", err)
	}
	if subscribed {
		return domain.Link{}, errors.New("link already tracked")
	}

	link, err := s.storage.AddLink(ctx, chatID, url, tags)
	if err != nil {
		return domain.Link{}, fmt.Errorf("adding link: %w", err)
	}

	if err = s.cache.InvalidateLinks(ctx, chatID); err != nil {
		slog.Warn("failed to invalidate cache on add", "error", err)
	}

	return link, nil
}

func (s *Service) RemoveLink(ctx context.Context, chatID int64, url string) (domain.Link, error) {
	slog.Debug("removing link", "chatID", chatID, "url", url)

	exists, err := s.storage.ChatExists(ctx, chatID)
	if err != nil {
		return domain.Link{}, fmt.Errorf("check chat existence: %w", err)
	}
	if !exists {
		slog.Warn("cannot remove link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	link, err := s.storage.RemoveLink(ctx, chatID, url)
	if err != nil {
		return domain.Link{}, fmt.Errorf("removing link: %w", err)
	}

	if err = s.cache.InvalidateLinks(ctx, chatID); err != nil {
		slog.Warn("failed to invalidate cache on remove", "error", err)
	}

	return link, nil
}

func (s *Service) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	slog.Debug("getting links", "chatID", chatID)

	exists, err := s.storage.ChatExists(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("check chat existence: %w", err)
	}
	if !exists {
		slog.Warn("cannot get links", "chatID", chatID, "error", errChatInstRegistered)
		return nil, errChatInstRegistered
	}

	cached, errGetCache := s.cache.GetLinks(ctx, chatID, limit, offset)
	if errGetCache != nil {
		slog.Warn("cannot get links from cache", "error", errGetCache)
	}
	if len(cached) != 0 {
		return cached, nil
	}

	links, err := s.storage.GetLinks(ctx, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("getting links: %w", err)
	}

	if offset == 0 && limit == s.defaultLimit && len(links) > 0 {
		if err = s.cache.SetLinks(ctx, chatID, limit, offset, links); err != nil {
			slog.Warn("failed to set cache", "error", err)
		}
	}

	return links, nil
}

func (s *Service) RegisterChat(ctx context.Context, chatID int64) error {
	slog.Debug("registering chat", "chatID", chatID)

	exists, err := s.storage.ChatExists(ctx, chatID)
	if err != nil {
		return fmt.Errorf("check chat existence: %w", err)
	}
	if exists {
		slog.Warn("cannot register chat", "chatID", chatID, "error", "chat already registered")
		return errors.New("chat already registered")
	}

	if err = s.storage.RegisterChat(ctx, chatID); err != nil {
		return fmt.Errorf("register chat: %w", err)
	}

	return nil
}

func (s *Service) DeleteChat(ctx context.Context, chatID int64) error {
	slog.Debug("deleting chat", "chatID", chatID)

	exists, err := s.storage.ChatExists(ctx, chatID)
	if err != nil {
		return fmt.Errorf("check chat existence: %w", err)
	}
	if !exists {
		slog.Warn("cannot delete chat", "chatID", chatID, "error", errChatInstRegistered)
		return errChatInstRegistered
	}

	if err = s.storage.DeleteChat(ctx, chatID); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}

	if err = s.cache.InvalidateLinks(ctx, chatID); err != nil {
		slog.Warn("failed to invalidate cache on delete chat", "error", err)
	}

	return nil
}

func (s *Service) CheckUpdates(ctx context.Context, interval time.Duration) {
	offset := defaultOffset
	var allErrors []domain.CheckResult

	for {
		links, err := s.storage.GetLinksWithInterval(ctx, s.batchSize, offset, interval)
		if err != nil {
			slog.Error("failed to get links", "error", err)
			return
		}

		if len(links) == 0 {
			break
		}

		errors := s.processBatch(ctx, links)
		allErrors = append(allErrors, errors...)

		offset += s.batchSize
	}

	if len(allErrors) > 0 {
		s.sendErrorReport(ctx, allErrors)
	}
}

func (s *Service) processBatch(ctx context.Context, links []domain.Link) []domain.CheckResult {
	chunkSize := (len(links) + s.workerCount - 1) / s.workerCount
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []domain.CheckResult

	slog.Debug("processing batch", "total_links", len(links), "worker_count", s.workerCount, "chunk_size", chunkSize)

	for i := range s.workerCount {
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(links) {
			break
		}
		if end > len(links) {
			end = len(links)
		}

		wg.Add(1)
		go func(chunk []domain.Link, workerID int) {
			defer wg.Done()
			slog.Debug("worker started", "worker_id", workerID, "chunk_size", len(chunk))

			for _, link := range chunk {
				result := s.checkLink(ctx, link)
				if result.Error != nil {
					mu.Lock()
					errors = append(errors, result)
					mu.Unlock()
				}
			}

			slog.Debug("worker finished", "worker_id", workerID)
		}(links[start:end], i)
	}

	wg.Wait()
	return errors
}

func (s *Service) checkLink(ctx context.Context, link domain.Link) domain.CheckResult {
	result := domain.CheckResult{
		Link:        link,
		ProcessedAt: time.Now(),
	}

	events, err := s.client.Check(ctx, link)
	if err != nil {
		result.Error = fmt.Errorf("check failed: %w", err)
		slog.Warn("error during checking link", "url", link.URL, "error", err)
		return result
	}

	if err = s.storage.UpdateLastChecked(ctx, link.URL, time.Now()); err != nil {
		slog.Warn("failed to update last checked", "url", link.URL, "error", err)
	}

	if len(events) == 0 {
		return result
	}

	chatIDs, err := s.storage.GetSubscribers(ctx, link.URL)
	if err != nil {
		result.Error = fmt.Errorf("get subscribers failed: %w", err)
		slog.Warn("failed to get subscribers", "url", link.URL, "error", err)
		return result
	}

	var maxTime time.Time

	for _, event := range events {
		if event.OccurredAt.After(maxTime) {
			maxTime = event.OccurredAt
		}

		if err = s.notifier.SendUpdate(ctx, domain.LinkUpdate{
			ID:          link.ID,
			URL:         link.URL,
			Description: event.Description,
			ChatIDs:     chatIDs,
		}); err != nil {
			slog.Error("failed to send update", "url", link.URL, "error", err)
		}
	}

	if err = s.storage.UpdateTimestamp(ctx, link.URL, maxTime); err != nil {
		slog.Warn("failed to update timestamp", "url", link.URL, "error", err)
	}

	result.Events = events
	return result
}

func (s *Service) sendErrorReport(ctx context.Context, errors []domain.CheckResult) {
	errorsByChat := make(map[int64][]string)

	for _, err := range errors {
		subscribers, errSub := s.storage.GetSubscribers(ctx, err.Link.URL)
		if errSub != nil {
			slog.Warn("got error while getting subs", "link", err.Link.URL, "error", errSub)
			continue
		}
		for _, chatID := range subscribers {
			errorsByChat[chatID] = append(errorsByChat[chatID], fmt.Sprintf("- %s: %v", err.Link.URL, err.Error))
		}
	}

	for chatID, errList := range errorsByChat {
		report := fmt.Sprintf("Ошибки при проверке ссылок\n"+
			"Не удалось обработать %d ссылок:\n%s",
			len(errList), joinErrors(errList))

		err := s.notifier.SendUpdate(ctx, domain.LinkUpdate{
			Description: report,
			ChatIDs:     []int64{chatID},
		})
		if err != nil {
			slog.Warn("got error while sending update", "chatdID", chatID, "error", err)
			continue
		}
	}
}

func joinErrors(errors []string) string {
	var builder strings.Builder
	for i, s := range errors {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(s)
	}
	return builder.String()
}
