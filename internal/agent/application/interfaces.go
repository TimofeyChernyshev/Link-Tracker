package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Notifier interface {
	SendUpdate(ctx context.Context, updatedLink domain.ProcessedUpdate) error
}
