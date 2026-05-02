package benchmark

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type NoopCache struct{}

func (n *NoopCache) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	return nil, nil
}

func (n *NoopCache) SetLinks(ctx context.Context, chatID int64, links []domain.Link) error {
	return nil
}

func (n *NoopCache) InvalidateLinks(ctx context.Context, chatID int64) error {
	return nil
}
