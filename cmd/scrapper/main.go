package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/cache"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	linkchecker "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/link_checker"
	scrappernotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
	scrapperhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/receiver/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/scheduler"
)

type ScrapperApp struct {
	cfg         *config.Config
	repo        repository.Repository
	notifier    scrappernotifier.Notifier
	linkService *application.Service
	scheduler   *scheduler.Scheduler
	server      *scrapperhttp.Server
	cache       *cache.ValkeyClient
}

func main() {
	setLogger()

	if err := godotenv.Load(".env.scrapper"); err != nil {
		slog.Warn("no .env.scrapper file found")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	app, err := buildApp(cfg)
	if err != nil {
		slog.Error("failed to build app", "error", err)
		os.Exit(1)
	}

	if err = app.run(); err != nil {
		slog.Error("app runtime error", "error", err)
		os.Exit(1)
	}
}

func buildApp(cfg *config.Config) (*ScrapperApp, error) {
	if err := runMigrations(cfg); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}

	repo, err := repository.NewRepository(cfg, fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName))
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	linkChecker := linkchecker.NewLinkChecker(
		"link-tracker", cfg.APIBatchSize, cfg.CheckerPreviewLen,
		cfg.GighubBaseURL, cfg.StackBaseURL,
		cfg.LinkCheckerTimeout, cfg.GithubTimeout, cfg.StackTimeout,
	)

	notifier, err := scrappernotifier.NewNotifier(cfg.NotifierConfig)
	if err != nil {
		repo.Close()
		return nil, fmt.Errorf("failed to create notifier: %w", err)
	}

	cache, err := cache.NewValkeyClient(cfg.ValkeyAddresses, cfg.ValkeyPassword, cfg.ValkeyTTL,
		cfg.ValkeyPoolSize, cfg.ValkeyMaxRetries, cfg.ValkeyMinRetryBackoff,
		cfg.ValkeyMaxRetryBackoff, cfg.ValkeyPingTime, cfg.ValkeyClusterMode, cfg.ValkeyScanCount)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache client: %w", err)
	}

	linkService := application.NewLinkService(linkChecker, notifier, repo, cache, cfg.BatchSize, cfg.WorkerCount, cfg.DefaultLimit)

	sched, err := scheduler.New(cfg.CheckInterval, linkService)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	server := scrapperhttp.NewServer(cfg.ScrapperPort, linkService, cfg.DefaultLimit, cfg.MaxLimit)

	return &ScrapperApp{
		cfg:         cfg,
		repo:        repo,
		notifier:    notifier,
		linkService: linkService,
		scheduler:   sched,
		server:      server,
		cache:       cache,
	}, nil
}

func (a *ScrapperApp) run() error {
	defer a.cleanup()

	a.scheduler.Start()
	slog.Info("scheduler started", "interval", a.cfg.CheckInterval)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	go func() {
		slog.Info("starting scrapper server", "port", a.cfg.ScrapperPort)
		if err := a.server.Start(); err != nil {
			errChan <- err
		}
	}()

	select {
	case sig := <-sigChan:
		slog.Info("received signal", "signal", sig)
	case err := <-errChan:
		slog.Error("server error", "error", err)
	}

	return a.shutdown()
}

func (a *ScrapperApp) shutdown() error {
	slog.Info("shutting down scrapper")

	if err := a.scheduler.Stop(); err != nil {
		slog.Error("scheduler stop error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		slog.Error("server stop error", "error", err)
	}

	if err := a.cache.Close(); err != nil {
		slog.Error("cache stop error", "error", err)
	}

	slog.Info("scrapper stopped")
	return nil
}

func (a *ScrapperApp) cleanup() {
	if a.repo != nil {
		a.repo.Close()
	}
}

func setLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func runMigrations(cfg *config.Config) error {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	m, err := migrate.New(
		"file://migrations",
		connString,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
