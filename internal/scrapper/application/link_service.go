package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

var (
	errChatInstRegistered = errors.New("chat isn't registered")

	linksForUpdateLimit = 20
	defaultOffset       = 0
)

type Service struct {
	client   Client
	notifier Notifier
	storage  Storage
}

func NewLinkService(client Client, notifier Notifier, storage Storage) *Service {
	return &Service{client: client, notifier: notifier, storage: storage}
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

	link, err := s.storage.AddLink(ctx, chatID, url, tags)
	if err != nil {
		return domain.Link{}, fmt.Errorf("adding link: %w", err)
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

	links, err := s.storage.GetLinks(ctx, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("getting links: %w", err)
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

	return nil
}

func (s *Service) CheckUpdates(ctx context.Context) {
	offset := defaultOffset

	for {
		links, err := s.storage.GetAllLinks(ctx, linksForUpdateLimit, offset)
		if err != nil {
			slog.Error("failed to get links", "error", err)
			return
		}

		if len(links) == 0 {
			break
		}

		for _, link := range links {
			s.checkLink(ctx, link)
		}

		offset += linksForUpdateLimit
	}
}

func (s *Service) checkLink(ctx context.Context, link domain.Link) {
	events, err := s.client.Check(ctx, link)
	if err != nil {
		slog.Warn("error during checking link", "url", link.URL, "error", err)
		return
	}

	if err = s.storage.UpdateLastChecked(ctx, link.URL, time.Now()); err != nil {
		slog.Warn("failed to update last checked", "url", link.URL, "error", err)
	}

	if len(events) == 0 {
		return
	}

	chatIDs, err := s.storage.GetSubscribers(ctx, link.URL)
	if err != nil {
		slog.Warn("failed to get subscribers", "url", link.URL, "error", err)
		return
	}

	var maxTime time.Time

	for _, event := range events {
		if event.OccurredAt.After(maxTime) {
			maxTime = event.OccurredAt
		}

		if err := s.notifier.SendUpdate(ctx, domain.LinkUpdate{
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
}
