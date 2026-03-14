package storage

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Storage struct {
	mu              sync.RWMutex
	registeredChats map[int64]struct{}
	links           map[string]*domain.Link       // url -> Link
	subs            map[string]map[int64]struct{} // url -> chatID
}

func New() *Storage {
	return &Storage{
		registeredChats: make(map[int64]struct{}),
		links:           make(map[string]*domain.Link),
		subs:            make(map[string]map[int64]struct{}),
	}
}

func (s *Storage) RegisterChat(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registeredChats[chatID] = struct{}{}
}

func (s *Storage) DeleteChat(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.registeredChats, chatID)

	for url, set := range s.subs {
		delete(set, chatID)

		if len(set) == 0 {
			delete(s.subs, url)
			delete(s.links, url)
		}
	}
}

func (s *Storage) ChatExists(chatID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.registeredChats[chatID]

	return ok
}

func (s *Storage) AddLink(chatID int64, URL string, tags []string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, exists := s.links[URL]
	if !exists {
		link = &domain.Link{
			URL:       URL,
			Tags:      tags,
			UpdatedAt: time.Now(),
		}

		s.links[URL] = link
	}

	if _, ok := s.subs[URL]; !ok {
		s.subs[URL] = make(map[int64]struct{})
	}

	if _, ok := s.subs[URL][chatID]; ok {
		slog.Warn("link already tracked", "chatID", chatID, "url", URL)
		return domain.Link{}, errors.New("link already tracked")
	}

	s.subs[URL][chatID] = struct{}{}

	return *link, nil
}

func (s *Storage) RemoveLink(chatID int64, URL string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, ok := s.subs[URL]
	if !ok {
		return domain.Link{}, errors.New("link not found")
	}

	delete(set, chatID)

	link := s.links[URL]

	if len(set) == 0 {
		delete(s.subs, URL)
		delete(s.links, URL)
	}

	return *link, nil
}

func (s *Storage) GetLinks(chatID int64) []domain.Link {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.Link

	for url, set := range s.subs {
		if _, ok := set[chatID]; ok {
			result = append(result, *s.links[url])
		}
	}

	return result
}

func (s *Storage) GetAllLinks() []domain.Link {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]domain.Link, 0, len(s.links))
	for _, l := range s.links {
		res = append(res, *l)
	}

	return res
}

func (s *Storage) GetSubscribers(URL string) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set := s.subs[URL]

	res := make([]int64, 0, len(set))
	for id := range set {
		res = append(res, id)
	}
	return res
}

func (s *Storage) UpdateTimestamp(URL string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.links[URL]; ok {
		l.UpdatedAt = t
	}
}
