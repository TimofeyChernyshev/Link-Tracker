package main

import (
	"context"
	"errors"
	"fmt"
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

const goroutines = 2

type BotApp struct {
	cfg            *config.Config
	metrics        *botmetrics.Metrics
	scrapperClient *clients.ScrapperClient
	botClient      *bot.Client
	dispatcher     *application.CommandDispatcher
	receiver       receiver.Receiver
}

func main() {
	setLogger()

	if err := godotenv.Load(".env.bot"); err != nil {
		slog.Warn("no .env.bot file found")
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
func buildApp(cfg *config.Config) (*BotApp, error) {
	metric := botmetrics.NewMetrics(cfg.BotMetricTick)

	httpClient := resilience.NewResilientHTTPClient(
		cfg.ScrapperRetryConfig,
		cfg.ScrapperCircuitBreakerConfig,
		cfg.ScrapperRateLimit,
		cfg.ScrapperTimeout,
	)
	scrapperClient := clients.NewScrapperClient(cfg.ScrapperBaseURL, httpClient, metric)

	botClient, err := bot.NewClient(
		cfg.TelegramToken,
		cfg.TelegramEndpoint,
		cfg.WorkerCount,
		cfg.SenderCount,
		cfg.JobsBufferSize,
		cfg.OutgoingBufferSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create bot client: %w", err)
	}

	dispatcher := setupDispatcher(botClient, scrapperClient, metric, cfg)
	botClient.SetCommands(dispatcher.GetCommands())

	receiver := setupReceiver(dispatcher, cfg, metric)

	return &BotApp{
		cfg:            cfg,
		metrics:        metric,
		scrapperClient: scrapperClient,
		botClient:      botClient,
		dispatcher:     dispatcher,
		receiver:       receiver,
	}, nil
}

func (a *BotApp) run() error {
	metricsShutdown, err := a.metrics.RunMetricsServer(a.cfg.MetricPort)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()
		if err = metricsShutdown(ctx); err != nil {
			slog.Error("metrics server shutdown error", "error", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, goroutines)

	go func() {
		slog.Info("starting bot")
		if err = a.botClient.Start(); err != nil {
			errChan <- fmt.Errorf("bot start error: %w", err)
		}
	}()

	go func() {
		slog.Info("starting receiver (HTTP + Kafka)")
		if err = a.receiver.Start(context.Background()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("receiver start error: %w", err)
		}
	}()

	slog.Info("Bot started")

	go a.dispatcher.Run(context.Background())

	select {
	case sig := <-sigChan:
		slog.Info("received signal", "signal", sig)
	case err = <-errChan:
		slog.Error("runtime error", "error", err)
	}

	return a.shutdown()
}

func (a *BotApp) shutdown() error {
	slog.Info("shutting down bot")

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.receiver.Shutdown(ctx); err != nil {
		slog.Error("receiver shutdown error", "error", err)
	}
	if err := a.botClient.Stop(ctx); err != nil {
		slog.Error("bot stop error", "error", err)
	}

	slog.Info("bot stopped")
	return nil
}

func setupReceiver(d *application.CommandDispatcher, cfg *config.Config, metric *botmetrics.Metrics) receiver.Receiver {
	rateLimiter := resilience.NewRateLimiterMiddleware(cfg.RateLimiterConfig.RPS, cfg.RateLimiterConfig.Burst)

	httpServer := bothttp.NewServer(d, cfg.BotPort, rateLimiter.Middleware, metric.HTTPMiddleware)

	kafkaConsumer := botkafka.NewConsumer(
		d,
		cfg.CommonKafkaConfig.Brokers,
		cfg.KafkaConsumerConfig.Topic,
		cfg.KafkaConsumerConfig.GroupID,
		cfg.KafkaConsumerConfig.SessionTimeout,
		cfg.KafkaConsumerConfig.MinBytes,
		cfg.KafkaConsumerConfig.MaxBytes,
		cfg.DLQConfig.MaxRetries,
		cfg.DLQConfig.BatchSize,
		cfg.DLQConfig.RetryDelay,
		cfg.DLQConfig.BatchTimeout,
		cfg.DLQConfig.DLQTopic,
		metric,
	)

	return receiver.NewMultiReceiver([]receiver.Receiver{httpServer, kafkaConsumer})
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
	d.Register("/list", func() application.Command {
		return handlers.NewListHandler(scrapperClient, cfg.TimeoutListHandler)
	})

	return d
}

func setLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
