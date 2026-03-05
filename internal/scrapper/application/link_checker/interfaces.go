package linkchecker

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

type Storage interface {
	GetAllLinks() []domain.Link
	GetSubscribers(url string) []int64
	UpdateTimestamp(url string, t time.Time)
}
