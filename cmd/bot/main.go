package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/dispatcher"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
)

const shutdownTimeout = 30 * time.Second

func main() {
	// Создание логера
	setLogger()

	// Загрузка конфига
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Добавление команд в диспетчер
	d := dispatcher.NewCommandDispatcher(dispatcher.NewUnknownHandler())
	d.Register(dispatcher.NewStartHandler())
	d.Register(dispatcher.NewHelpHandler())

	bot, err := bot.NewBotClient(cfg.TelegramToken, d)
	if err != nil {
		slog.Error("cannot start bot", "error", err)
	}

	// Канал сигналов с размером 1
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Канал ошибок
	errChan := make(chan error, 1)

	// Запуск бота в горутине
	go func() {
		slog.Info("bot starting")
		if err = bot.Start(); err != nil {
			errChan <- err
		}
	}()

	// Ожидание сигнала о завершении или ошибку
	select {
	case sig := <-sigChan:
		slog.Info("Received signal", "signal", sig)
	case err = <-errChan:
		slog.Error("failed to start bot", "error", err)
		os.Exit(1)
	}

	// Остановка получения сигналов и закрытие канала
	signal.Stop(sigChan)
	close(sigChan)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err = bot.Stop(shutdownCtx); err != nil {
		slog.Error("error during shutdown", "error", err)
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
