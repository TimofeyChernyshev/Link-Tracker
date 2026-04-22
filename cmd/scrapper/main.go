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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	linkchecker "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/link_checker"
	scrappernotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
	scrapperhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/receiver/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/scheduler"
)

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

	if err = runMigrations(cfg); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	repo, err := repository.NewRepository(cfg, fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName))
	if err != nil {
		slog.Error("failed to create repository", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	// HTTP клиенты
	linkChecker := linkchecker.NewLinkChecker(
		"link-tracker", cfg.APIBatchSize, cfg.CheckerPreviewLen,
		cfg.GighubBaseURL, cfg.StackBaseURL,
		cfg.LinkCheckerTimeout, cfg.GithubTimeout, cfg.StackTimeout,
	)

	// Notifier для отправки уведомлений в Bot
	botNotifier, err := scrappernotifier.NewNotifier(cfg.NotifierConfig)
	if err != nil {
		slog.Error("failed to create bot notifier", "error", err)
		os.Exit(1)
	}

	// Сервис работы с ссылками
	linkService := application.NewLinkService(linkChecker, botNotifier, repo, cfg.BatchSize, cfg.WorkerCount)

	// Планировщик
	sched, err := scheduler.New(cfg.CheckInterval, linkService)

	// HTTP сервер для API Scrapper
	server := scrapperhttp.NewServer(cfg.ScrapperPort, linkService, cfg.DefaultLimit, cfg.MaxLimit)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	sched.Start()
	slog.Info("scheduler started", "interval", cfg.CheckInterval)

	go func() {
		slog.Info("starting scrapper server", "port", cfg.ScrapperPort)
		if err = server.Start(); err != nil {
			errChan <- err
		}
	}()

	select {
	case sig := <-sigChan:
		slog.Info("received signal", "signal", sig)
	case err = <-errChan:
		slog.Error("server error", "error", err)
	}

	slog.Info("shutting down scrapper")

	if err = sched.Stop(); err != nil {
		slog.Error("scheduler stop error", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err = server.Shutdown(ctx); err != nil {
		slog.Error("server stop error", "error", err)
	}

	slog.Info("scrapper stopped")
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
