package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
)

type LinkChecker interface {
	CheckUpdates(ctx context.Context)
}

type Scheduler struct {
	scheduler gocron.Scheduler
	cancel    context.CancelFunc
}

func New(interval time.Duration, checker LinkChecker) (*Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("creating new scheduler: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	_, err = s.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(func() {
			slog.Info("scheduler tick: checking links")
			checker.CheckUpdates(ctx)
		}),
	)

	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating new job: %w", err)
	}

	return &Scheduler{
		scheduler: s,
		cancel:    cancel,
	}, nil
}

func (s *Scheduler) Start() {
	s.scheduler.Start()
}

func (s *Scheduler) Stop() error {
	s.cancel()
	err := s.scheduler.Shutdown()
	return fmt.Errorf("stoping scheduler: %w", err)
}
