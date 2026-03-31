package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	ormrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/orm"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
)

type Repository interface {
	Close()

	RegisterChat(ctx context.Context, chatID int64) error
	DeleteChat(ctx context.Context, chatID int64) error
	ChatExists(ctx context.Context, chatID int64) (bool, error)

	AddLink(ctx context.Context, chatID int64, URL string, tags []string) (domain.Link, error)
	RemoveLink(ctx context.Context, chatID int64, URL string) (domain.Link, error)
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)
	GetAllLinks(ctx context.Context, limit, offset int) ([]domain.Link, error)
	GetSubscribers(ctx context.Context, url string) ([]int64, error)
	UpdateTimestamp(ctx context.Context, url string, t time.Time) error
	UpdateLastChecked(ctx context.Context, url string, t time.Time) error

	DB() *sql.DB
}

func NewRepository(cfg *config.Config, connString string) (Repository, error) {
	switch cfg.AccessType {
	case config.AccessTypeSQL:
		repo, err := sqlrepo.NewRepository(connString)
		if err != nil {
			return nil, fmt.Errorf("cannot create repository: %w", err)
		}

		return repo, nil
	case config.AccessTypeORM:
		repo, err := ormrepo.NewRepository(connString)
		if err != nil {
			return nil, fmt.Errorf("cannot create repository: %w", err)
		}

		return repo, nil
	default:
		return nil, fmt.Errorf("unknown access type: %s", cfg.AccessType)
	}
}
