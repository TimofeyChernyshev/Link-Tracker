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
)

type Service struct {
	client   Client
	notifier Notifier
	storage  Storage
}

func NewLinkService(client Client, notifier Notifier, storage Storage) *Service {
	return &Service{client: client, notifier: notifier, storage: storage}
}

func (s *Service) AddLink(chatID int64, url string, tags []string) (domain.Link, error) {
	slog.Debug("adding link", "chatID", chatID, "url", url, "tags", tags)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot add link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	link, err := s.storage.AddLink(chatID, url, tags)
	if err != nil {
		return domain.Link{}, fmt.Errorf("adding link: %w", err)
	}

	return link, nil
}

func (s *Service) RemoveLink(chatID int64, url string) (domain.Link, error) {
	slog.Debug("removing link", "chatID", chatID, "url", url)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot remove link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	link, err := s.storage.RemoveLink(chatID, url)
	if err != nil {
		return domain.Link{}, fmt.Errorf("removing link: %w", err)
	}

	return link, nil
}

func (s *Service) GetLinks(chatID int64) ([]domain.Link, error) {
	slog.Debug("getting links", "chatID", chatID)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot get links", "chatID", chatID, "error", errChatInstRegistered)
		return nil, errChatInstRegistered
	}

	links := s.storage.GetLinks(chatID)

	return links, nil
}

func (s *Service) RegisterChat(chatID int64) error {
	slog.Debug("registering chat", "chatID", chatID)

	if s.storage.ChatExists(chatID) {
		slog.Warn("cannot register chat", "chatID", chatID, "error", errors.New("chat already registered"))
		return errors.New("chat already registered")
	}

	s.storage.RegisterChat(chatID)

	return nil
}

func (s *Service) DeleteChat(chatID int64) error {
	slog.Debug("deleting chat", "chatID", chatID)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot delete chat", "chatID", chatID, "error", errChatInstRegistered)
		return errChatInstRegistered
	}

	s.storage.DeleteChat(chatID)

	return nil
}

func (s *Service) CheckUpdates(ctx context.Context) {
	links := s.storage.GetAllLinks()

	for _, link := range links {
		changed, desc, err := s.client.Check(ctx, link)
		if err != nil {
			slog.Warn("error during checking link", "link", link, "error", err)
			continue
		}
		if !changed {
			continue
		}

		s.storage.UpdateTimestamp(link.URL, time.Now())

		chatIDs := s.storage.GetSubscribers(link.URL)

		_ = s.notifier.SendUpdate(ctx, domain.LinkUpdate{
			ID:          link.ID,
			URL:         link.URL,
			Description: desc,
			ChatIDs:     chatIDs,
		})
	}
}
