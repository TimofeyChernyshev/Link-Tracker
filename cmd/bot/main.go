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
	botmetrics "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/metrics"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver"
	bothttp "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/http"
	botkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/receiver/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

const (
	goroutines = 2
)

func main() {
	setLogger()

	_ = godotenv.Load(".env.bot")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	metric := botmetrics.NewMetrics(cfg.BotMetricTick)
	metricsShutdown, err := metric.RunMetricsServer("9091")
	if err != nil {
		slog.Error("failed to start metrics server", "error", err)
		os.Exit(1)
	}

	httpClient := resilience.NewResilientHTTPClient(cfg.ScrapperRetryConfig, cfg.ScrapperCircuitBreakerConfig, cfg.ScrapperRateLimit, cfg.ScrapperTimeout)
	scrapperClient := clients.NewScrapperClient(cfg.ScrapperBaseURL, httpClient, metric)

	b, err := bot.NewClient(cfg.TelegramToken, cfg.TelegramEndpoint, cfg.WorkerCount, cfg.SenderCount, cfg.JobsBufferSize, cfg.OutgoingBufferSize)
	if err != nil {
		slog.Error("cannot start bot", "error", err)
		os.Exit(1)
	}

	d := setupDispatcher(b, scrapperClient, metric, cfg)

	b.SetCommands(d.GetCommands())

	receiver := setupReceiver(d, cfg, metric)

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

	slog.Info("Bot started")

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
	if err = metricsShutdown(shutdownCtx); err != nil {
		slog.Error("metrics server shutdown error", "error", err)
	}

	slog.Info("bot stoped")
}

func setupReceiver(d *application.CommandDispatcher, cfg *config.Config, metric *botmetrics.Metrics) receiver.Receiver {
	rateLimiter := resilience.NewRateLimiterMiddleware(cfg.RateLimiterConfig.RPS, cfg.RateLimiterConfig.Burst)

	httpServer := bothttp.NewServer(d, cfg.BotPort, rateLimiter.Middleware, metric.HTTPMiddleware)
	kafkaConsumer := botkafka.NewConsumer(
		d, cfg.CommonKafkaConfig.Brokers, cfg.KafkaConsumerConfig.Topic, cfg.KafkaConsumerConfig.GroupID,
		cfg.KafkaConsumerConfig.SessionTimeout, cfg.KafkaConsumerConfig.MinBytes, cfg.KafkaConsumerConfig.MaxBytes,
		cfg.DLQConfig.MaxRetries, cfg.DLQConfig.BatchSize, cfg.DLQConfig.RetryDelay,
		cfg.DLQConfig.BatchTimeout, cfg.DLQConfig.DLQTopic,
		metric,
	)

	receiver := receiver.NewMultiReceiver([]receiver.Receiver{httpServer, kafkaConsumer})

	return receiver
}

func setLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func setupDispatcher(bot application.Bot, scrapperClient *clients.ScrapperClient, metric *botmetrics.Metrics, cfg *config.Config) *application.CommandDispatcher {
	d := application.NewCommandDispatcher(handlers.NewUnknownHandler(), bot, metric)
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
