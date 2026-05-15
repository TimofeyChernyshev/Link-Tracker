package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/handlers"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/clients"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver"
)

const (
	goroutines = 2
)

func main() {
	// Создание логера
	setLogger()

	// Загрузка конфига
	_ = godotenv.Load(".env.bot")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	scrapperClient := clients.NewScrapperClient(cfg.ScrapperBaseURL, cfg.ScrapperTimeout)

	b, err := bot.NewClient(cfg.TelegramToken, cfg.TelegramEndpoint, cfg.WorkerCount, cfg.SenderCount, cfg.JobsBufferSize, cfg.OutgoingBufferSize)
	if err != nil {
		slog.Error("cannot start bot", "error", err)
		os.Exit(1)
	}

	d := setupDispatcher(b, scrapperClient, cfg)

	b.SetCommands(d.GetCommands())

	receiver, err := receiver.NewReceiver(cfg.ReceiverConfig, d)
	if err != nil {
		slog.Error("cannot create receiver", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Канал ошибок
	errChan := make(chan error, goroutines)

	// Запуск бота в горутине
	go func() {
		slog.Info("bot starting")
		if err = b.Start(); err != nil {
			errChan <- err
		}
	}()

	go func() {
		err = receiver.Start(context.Background())
		if !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	d.Run(ctx)

	// Ожидание сигнала о завершении или ошибку
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err = <-errChan:
		slog.Error("runtime error", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err = receiver.Shutdown(shutdownCtx); err != nil {
		slog.Error("error during shutdown server", "error", err)
	}
	if err = b.Stop(shutdownCtx); err != nil {
		slog.Error("error during shutdown bot", "error", err)
	}

	slog.Info("bot stoped")
}

func setLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func setupDispatcher(bot application.Bot, scrapperClient *clients.ScrapperClient, cfg *config.Config) *application.CommandDispatcher {
	d := application.NewCommandDispatcher(handlers.NewUnknownHandler(), bot)
	start := handlers.NewStartHandler(scrapperClient, cfg.TimeoutStartHandler)
	d.Register(start.Name(), func() application.Command { return start })
	help := handlers.NewHelpHandler()
	d.Register(help.Name(), func() application.Command { return help })
	d.Register("/track", func() application.Command {
		return handlers.NewTrackHandler(scrapperClient, cfg.TimeoutSaveLink, cfg.TimeoutCheckLink)
	})
	d.Register("/untrack", func() application.Command {
		return handlers.NewUntrackHandler(scrapperClient, cfg.TimeoutUntrackHandler)
	})
	d.Register("/list", func() application.Command { return handlers.NewListHandler(scrapperClient, cfg.TimeoutListHandler) })

	return d
}
