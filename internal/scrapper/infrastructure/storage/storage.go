package storage

import (
	"errors"
	"log/slog"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Storage struct {
	mu        sync.RWMutex
	userLinks map[int64]map[string]domain.Link // chatID -> map[url]Link
}

func New() *Storage {
	return &Storage{
		userLinks: make(map[int64]map[string]domain.Link),
	}
}

func (s *Storage) RegisterChat(chatId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userLinks[chatId] = make(map[string]domain.Link)
}

func (s *Storage) DeleteChat(chatId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.userLinks, chatId)
}

func (s *Storage) ChatExists(chatId int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.userLinks[chatId]; exists {
		return true
	} else {
		return false
	}
}

func (s *Storage) AddLink(chatId int64, url string, tags []string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.userLinks[chatId][url]; exists {
		slog.Warn("link already tracked", "chatID", chatId, "url", url)
		return domain.Link{}, errors.New("link already tracked")
	}

	link := domain.Link{URL: url, Tags: tags}
	s.userLinks[chatId][url] = link

	return link, nil
}

func (s *Storage) RemoveLink(chatId int64, url string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userLinks := s.userLinks[chatId]

	removedLink, exists := userLinks[url]
	if !exists {
		slog.Warn("link not found", "chatID", chatId, "url", url)
		return domain.Link{}, errors.New("link not found")
	}
	delete(userLinks, url)

	return removedLink, nil
}

func (s *Storage) GetLinks(chatID int64) []domain.Link {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.Link
	if userLinks, exists := s.userLinks[chatID]; exists {
		for _, link := range userLinks {
			result = append(result, link)
		}
	}
	return result
}
