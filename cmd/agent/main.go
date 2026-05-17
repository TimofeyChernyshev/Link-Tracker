package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/config"
	kafkanotifier "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/notifier"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/receiver"
)

type AgentApp struct {
	cfg          *config.Config
	notifier     *kafkanotifier.KafkaNotifier
	agentService *application.AgentService
	consumer     *receiver.Consumer
}

func main() {
	setLogger()

	if err := godotenv.Load(".env.agent"); err != nil {
		slog.Warn("no .env.agent file found")
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

func buildApp(cfg *config.Config) (*AgentApp, error) {
	kafkaNotifier := kafkanotifier.NewKafkaNotifier(
		cfg.KafkaNotifierConfig.Topic,
		cfg.KafkaNotifierConfig.Compression, cfg.CommonKafkaConfig.Brokers,
		cfg.KafkaNotifierConfig.BatchSize, cfg.KafkaNotifierConfig.RequiredAcks, cfg.KafkaNotifierConfig.BatchTimeout,
	)

	agentService := application.NewAgentService(cfg.FilterStopWords, cfg.FilterExcludedAuthors, cfg.FilterMinLength, cfg.SummarizationThreshold, kafkaNotifier)

	consumer := receiver.NewConsumer(
		agentService, cfg.CommonKafkaConfig.Brokers, cfg.KafkaConsumerConfig.Topic,
		cfg.KafkaConsumerConfig.GroupID, cfg.KafkaConsumerConfig.SessionTimeout, cfg.KafkaConsumerConfig.MinBytes,
		cfg.KafkaConsumerConfig.MaxBytes, cfg.DLQConfig.MaxRetries, cfg.DLQConfig.BatchSize,
		cfg.DLQConfig.RetryDelay, cfg.DLQConfig.BatchTimeout, cfg.DLQConfig.DLQTopic,
	)

	return &AgentApp{
		cfg:      cfg,
		notifier: kafkaNotifier,
		consumer: consumer,
	}, nil
}

func (a *AgentApp) run() error {
	slog.Info("starting agent service")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error, 1)

	ctx := context.Background()

	go func() {
		if err := a.consumer.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	slog.Info("Agent started")

	select {
	case sig := <-sigChan:
		slog.Info("received signal", "signal", sig)
	case err := <-errChan:
		slog.Error("consumer error", "error", err)
	}

	return a.shutdown()
}

func (a *AgentApp) shutdown() error {
	slog.Info("shutting down agent")

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.notifier.Close(); err != nil {
		slog.Error("server stop error", "error", err)
	}

	if err := a.consumer.Shutdown(ctx); err != nil {
		slog.Error("cache stop error", "error", err)
	}

	slog.Info("agent stopped")
	return nil
}

func setLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
