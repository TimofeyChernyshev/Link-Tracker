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

func (s *Storage) RegisterChat(chatId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registeredChats[chatId] = struct{}{}
}

func (s *Storage) DeleteChat(chatId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.registeredChats, chatId)

	for url, set := range s.subs {
		delete(set, chatId)

		if len(set) == 0 {
			delete(s.subs, url)
			delete(s.links, url)
		}
	}
}

func (s *Storage) ChatExists(chatId int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.registeredChats[chatId]

	return ok
}

func (s *Storage) AddLink(chatId int64, url string, tags []string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, exists := s.links[url]
	if !exists {
		link = &domain.Link{
			URL:       url,
			Tags:      tags,
			UpdatedAt: time.Now(),
		}

		s.links[url] = link
	}

	if _, ok := s.subs[url]; !ok {
		s.subs[url] = make(map[int64]struct{})
	}

	if _, ok := s.subs[url][chatId]; ok {
		slog.Warn("link already tracked", "chatID", chatId, "url", url)
		return domain.Link{}, errors.New("link already tracked")
	}

	s.subs[url][chatId] = struct{}{}

	return *link, nil
}

func (s *Storage) RemoveLink(chatId int64, url string) (domain.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, ok := s.subs[url]
	if !ok {
		return domain.Link{}, errors.New("link not found")
	}

	delete(set, chatId)

	link := s.links[url]

	if len(set) == 0 {
		delete(s.subs, url)
		delete(s.links, url)
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

func (s *Storage) GetSubscribers(url string) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set := s.subs[url]

	res := make([]int64, 0, len(set))
	for id := range set {
		res = append(res, id)
	}
	return res
}

func (s *Storage) UpdateTimestamp(url string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.links[url]; ok {
		l.UpdatedAt = t
	}
}
