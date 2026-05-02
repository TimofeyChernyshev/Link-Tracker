package benchmark

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type NoopCache struct{}

func (n *NoopCache) GetLinks(_ context.Context, _ int64) ([]domain.Link, error) {
	return nil, nil
}

func (n *NoopCache) SetLinks(_ context.Context, _ int64, _ []domain.Link) error {
	return nil
}

func (n *NoopCache) InvalidateLinks(_ context.Context, _ int64) error {
	return nil
}
