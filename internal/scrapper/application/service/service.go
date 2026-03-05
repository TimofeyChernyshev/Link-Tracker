package service

import (
	"errors"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

var (
	errChatInstRegistered = errors.New("chat isn't registered")
)

type Service struct {
	storage Storage
}

func New(storage Storage) *Service {
	return &Service{storage: storage}
}

func (s *Service) AddLink(chatID int64, url string, tags []string) (domain.Link, error) {
	slog.Debug("adding link", "chatID", chatID, "url", url, "tags", tags)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot add link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	return s.storage.AddLink(chatID, url, tags)
}

func (s *Service) RemoveLink(chatID int64, url string) (domain.Link, error) {
	slog.Debug("removing link", "chatID", chatID, "url", url)

	if !s.storage.ChatExists(chatID) {
		slog.Warn("cannot remove link", "chatID", chatID, "error", errChatInstRegistered)
		return domain.Link{}, errChatInstRegistered
	}

	return s.storage.RemoveLink(chatID, url)
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
