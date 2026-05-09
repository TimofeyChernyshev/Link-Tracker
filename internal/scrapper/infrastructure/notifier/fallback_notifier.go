package scrappernotifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Notifier interface {
	SendUpdate(ctx context.Context, upd domain.LinkUpdate) error
	Close() error
}

type FallbackNotifier struct {
	notifiers []Notifier
}

// NewFallbackNotifier - создает новый Notifier сервиса Bot
//
// notifiers - реализации интерфейса Notifier в порядке приоритета.
// Сначала будет использоваться notifier с индексом 0, если он не доступен, то 1 и т.д.
func NewFallbackNotifier(notifiers []Notifier) *FallbackNotifier {
	return &FallbackNotifier{
		notifiers: notifiers,
	}
}

func (f *FallbackNotifier) SendUpdate(ctx context.Context, upd domain.LinkUpdate) error {
	var lastErr error

	for i, n := range f.notifiers {
		err := n.SendUpdate(ctx, upd)
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("notifier failed", "level", i, "id", upd.ID, "error", err)
	}

	slog.Error("all notifiers failed", "id", upd.ID, "error", lastErr)
	return fmt.Errorf("all notifiers failed: %w", lastErr)
}

func (f *FallbackNotifier) Close() error {
	var errs []error
	for _, n := range f.notifiers {
		if err := n.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
