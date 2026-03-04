package service

import (
	"errors"

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
	if !s.storage.ChatExists(chatID) {
		return domain.Link{}, errChatInstRegistered
	}

	return s.storage.AddLink(chatID, url, tags)
}

func (s *Service) RemoveLink(chatID int64, url string) (domain.Link, error) {
	if !s.storage.ChatExists(chatID) {
		return domain.Link{}, errChatInstRegistered
	}

	return s.storage.RemoveLink(chatID, url)
}

func (s *Service) GetLinks(chatID int64) ([]domain.Link, error) {
	if !s.storage.ChatExists(chatID) {
		return nil, errChatInstRegistered
	}

	links := s.storage.GetLinks(chatID)

	return links, nil
}

func (s *Service) RegisterChat(chatID int64) error {
	if s.storage.ChatExists(chatID) {
		return errors.New("chat already registered")
	}

	s.storage.RegisterChat(chatID)

	return nil
}

func (s *Service) DeleteChat(chatID int64) error {
	if !s.storage.ChatExists(chatID) {
		return errChatInstRegistered
	}

	s.storage.DeleteChat(chatID)

	return nil
}
