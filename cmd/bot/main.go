package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/dispatcher"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/service"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/clients"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	bot_server "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/http"
)

const shutdownTimeout = 30 * time.Second

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

	scrapperClient := clients.NewScrapperClient(cfg.ScrapperBaseURL)

	// Добавление команд в диспетчер
	d := dispatcher.NewCommandDispatcher(dispatcher.NewUnknownHandler())
	start := dispatcher.NewStartHandler(scrapperClient)
	d.Register(start.Name(), func() dispatcher.Command { return start })
	help := dispatcher.NewHelpHandler()
	d.Register(help.Name(), func() dispatcher.Command { return help })
	d.Register("/track", func() dispatcher.Command { return dispatcher.NewTrackHandler(scrapperClient) })
	d.Register("/untrack", func() dispatcher.Command { return dispatcher.NewUntrackHandler(scrapperClient) })
	d.Register("/list", func() dispatcher.Command { return dispatcher.NewListHandler(scrapperClient) })

	bot, err := bot.NewClient(cfg.TelegramToken, d)
	if err != nil {
		slog.Error("cannot start bot", "error", err)
	}

	service := service.New(bot)

	server := bot_server.NewServer(service, cfg.BotPort)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Канал ошибок
	errChan := make(chan error, 2)

	// Запуск бота в горутине
	go func() {
		slog.Info("bot starting")
		if err = bot.Start(); err != nil {
			errChan <- err
		}
	}()

	go func() {
		slog.Info("http server for bot starting", "port", cfg.BotPort)
		err := server.Start()
		if !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	go func() {
		slog.Info("http server for bot starting", "port", cfg.BotPort)
		err := server.Start()
		if !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	// Ожидание сигнала о завершении или ошибку
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errChan:
		slog.Error("runtime error", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("error during shutdown server", "error", err)
	}
	if err := bot.Stop(shutdownCtx); err != nil {
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
