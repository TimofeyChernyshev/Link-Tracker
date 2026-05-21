package receiver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

type Receiver interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type MultiReceiver struct {
	receivers []Receiver
	wg        sync.WaitGroup
}

// NewMultiReceiver - запускает все переданные Receiver
func NewMultiReceiver(receivers []Receiver) *MultiReceiver {
	return &MultiReceiver{
		receivers: receivers,
	}
}

func (m *MultiReceiver) Start(ctx context.Context) error {
	var startedCount int
	var mu sync.Mutex

	for i, r := range m.receivers {
		m.wg.Add(1)
		go func(idx int, rec Receiver) {
			defer m.wg.Done()

			slog.Info("starting receiver", "priority", idx)
			if err := rec.Start(ctx); err != nil {
				slog.Error("receiver failed", "priority", idx, "error", err)
				return
			}

			mu.Lock()
			startedCount++
			mu.Unlock()
			slog.Info("receiver started successfully", "priority", idx)
		}(i, r)
	}

	m.wg.Wait()

	if startedCount == 0 {
		return errors.New("all receivers failed to start")
	}

	slog.Info("multi receiver started", "successful_count", startedCount, "total", len(m.receivers))
	return nil
}

func (m *MultiReceiver) Shutdown(ctx context.Context) error {
	var errs []error
	for _, r := range m.receivers {
		if err := r.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
