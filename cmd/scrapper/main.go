package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	botclient "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/bot_client"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
	linkchecker "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/link_checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/scheduler"
	scrapperhttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/server/http"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/storage"
)

const (
	shutdownTimeout = 30 * time.Second
	checkInterval   = 5 * time.Minute
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

	memStorage := storage.New()

	// HTTP клиенты
	linkChecker := linkchecker.NewLinkChecker()

	// Notifier для отправки уведомлений в Bot
	botNotifier := botclient.NewBotClient(cfg.BotBaseURL)

	// Сервис работы с ссылками
	linkService := application.NewLinkService(linkChecker, botNotifier, memStorage)

	// Планировщик
	sched, err := scheduler.New(checkInterval, linkService)

	// HTTP сервер для API Scrapper
	server := scrapperhttp.NewServer(cfg.ScrapperPort, linkService)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	sched.Start()
	slog.Info("scheduler started", "interval", checkInterval)

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

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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
