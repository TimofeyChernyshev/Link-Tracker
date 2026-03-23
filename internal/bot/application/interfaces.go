package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type LinkService interface {
	AddLink(ctx context.Context, chatID int64, url string, tags []string) error
	RemoveLink(ctx context.Context, chatID int64, url string) error
	GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error)

	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
}

type Command interface {
	Name() string
	Description() string

	Execute(msg *domain.Message) (resp *domain.Response, done bool, err error)
}

type Bot interface {
	Receive(ctx context.Context) (*domain.Message, error)
	SendMessage(response *domain.Response)
}
