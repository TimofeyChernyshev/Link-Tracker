package integration

import (
	"context"
	"fmt"
	"strings"

	"github.com/testcontainers/testcontainers-go"
)

func StartScrapperWithKafka(ctx context.Context, networkName, topic string, brokers []string) (testcontainers.Container, string, error) {
	brokersStr := strings.Join(brokers, ",")

	req := testcontainers.ContainerRequest{
		Image:        "link-tracker-scrapper",
		ExposedPorts: []string{"8081/tcp"},
		Env: map[string]string{
			"SCRAPPER_PORT":       "8081",
			"KAFKA_UPDATES_TOPIC": topic,
			"KAFKA_BROKERS":       brokersStr,
			"KAFKA_COMPRESSION":   "snappy",

			"ACCESS_TYPE": "sql",
			"DB_USER":     "postgres",
			"DB_PASSWORD": "postgres",
			"DB_HOST":     "postgres",
			"DB_PORT":     "5432",
			"DB_NAME":     "linktracker",

			"GITHUB_BASE_URL": "https://api.github.com/repos",
			"STACK_BASE_URL":  "https://api.stackexchange.com/2.3",

			"VALKEY_ADDRESSES":    "valkey-node-1:6379,valkey-node-2:6379,valkey-node-3:6379",
			"VALKEY_CLUSTER_MODE": "true",

			"BOT_BASE_URL": "123",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"scrapper"},
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started:          true,
		ContainerRequest: req,
	})
	if err != nil {
		return nil, "", fmt.Errorf("starting scrapper: %w", err)
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8081/tcp")
	url := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return container, url, nil
}

func StartBotWithKafka(ctx context.Context, networkName, telegramURL, topic, dlqTopic string, brokers []string) (testcontainers.Container, string, error) {
	brokersStr := strings.Join(brokers, ",")

	req := testcontainers.ContainerRequest{
		Image:        "link-tracker-bot",
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"TELEGRAM_TOKEN":        "test",
			"TELEGRAM_API_URL":      telegramURL,
			"SCRAPPER_BASE_URL":     "http://scrapper:8081",
			"BOT_PORT":              "8080",
			"KAFKA_UPDATES_TOPIC":   topic,
			"KAFKA_BROKERS":         brokersStr,
			"KAFKA_GROUP_ID":        "bot-test-group",
			"KAFKA_SESSION_TIMEOUT": "10s",
			"KAFKA_MIN_BYTES":       "1",
			"KAFKA_MAX_BYTES":       "10485760",
			"KAFKA_DLQ_TOPIC":       dlqTopic,
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"bot"},
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started:          true,
		ContainerRequest: req,
	})
	if err != nil {
		return nil, "", fmt.Errorf("starting bot: %w", err)
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8080/tcp")
	url := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return container, url, nil
}
