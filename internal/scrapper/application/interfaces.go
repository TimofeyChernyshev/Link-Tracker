package application

import (
	"context"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Client interface {
	Check(ctx context.Context, link domain.Link) (updates []domain.Event, err error)
}

type Notifier interface {
	SendUpdate(ctx context.Context, updatedLink domain.LinkUpdate) error
}

// Storage - контракт хранилища ссылок
type Storage interface {
	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	ChatExists(ctx context.Context, chatID int64) (bool, error)

	IsSubscribed(ctx context.Context, chatID int64, url string) (bool, error)
	AddLink(ctx context.Context, chatID int64, URL string, tags []string) (domain.Link, error)
	RemoveLink(ctx context.Context, chatID int64, URL string) (domain.Link, error)
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)
	GetLinksWithInterval(ctx context.Context, limit, offset int, interval time.Duration) ([]domain.Link, error)
	GetSubscribers(ctx context.Context, url string) ([]int64, error)
	UpdateTimestamp(ctx context.Context, url string, t time.Time) error
	UpdateLastChecked(ctx context.Context, url string, t time.Time) error
}
