package application

import (
	"context"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Client interface {
	Check(ctx context.Context, link domain.Link) (changed bool, description string, err error)
}

type Notifier interface {
	SendUpdate(ctx context.Context, updatedLink domain.LinkUpdate) error
}

// Storage - контракт хранилища ссылок
type Storage interface {
	RegisterChat(chatID int64)
	DeleteChat(chatID int64)
	ChatExists(chatID int64) bool
	AddLink(chatID int64, URL string, tags []string) (domain.Link, error)
	RemoveLink(chatID int64, URL string) (domain.Link, error)
	GetLinks(chatID int64) []domain.Link
	UpdateTimestamp(url string, t time.Time)
	GetSubscribers(url string) []int64
	GetAllLinks() []domain.Link
}
