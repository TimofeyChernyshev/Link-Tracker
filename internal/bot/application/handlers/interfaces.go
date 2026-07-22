package handlers

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type LinkService interface {
	AddLink(ctx context.Context, chatID int64, url string, tags []string) error
	RemoveLink(ctx context.Context, chatID int64, url string) error
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)

	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
}
