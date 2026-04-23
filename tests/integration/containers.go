package integration

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartScrapper(ctx context.Context, networkName string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image: "link-tracker-scrapper",

		ExposedPorts: []string{"8081/tcp"},

		Env: map[string]string{
			"SCRAPPER_PORT":   "8081",
			"ACCESS_TYPE":     "sql",
			"DB_USER":         "postgres",
			"DB_PASSWORD":     "postgres",
			"DB_HOST":         "postgres",
			"DB_PORT":         "5432",
			"DB_NAME":         "linktracker",
			"GITHUB_BASE_URL": "https://api.github.com/repos",
			"STACK_BASE_URL":  "https://api.stackexchange.com/2.3",

			"NOTIFICATION_TYPE":       "http",
			"BOT_BASE_URL":            "http://bot:8080",
			"SCRAPPER_TO_BOT_TIMEOUT": "5s",
		},

		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"scrapper"},
		},

		WaitingFor: wait.ForListeningPort("8081/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started:          true,
		ContainerRequest: req,
	})
	if err != nil {
		return nil, "", fmt.Errorf("starting container: %w", err)
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8081/tcp")

	url := "http://" + host + ":" + mappedPort.Port()

	return container, url, nil
}

func StartBot(ctx context.Context, networkName string, telegramURL string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image: "link-tracker-bot",

		ExposedPorts: []string{"8080/tcp"},

		Env: map[string]string{
			"TELEGRAM_TOKEN":    "test",
			"TELEGRAM_API_URL":  telegramURL,
			"BOT_PORT":          "8080",
			"SCRAPPER_BASE_URL": "http://scrapper:8081",

			"NOTIFICATION_TYPE": "http",
		},

		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"bot"},
		},

		WaitingFor: wait.ForListeningPort("8080/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started:          true,
		ContainerRequest: req,
	})
	if err != nil {
		return nil, "", fmt.Errorf("starting container: %w", err)
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8080/tcp")

	url := "http://" + host + ":" + mappedPort.Port()

	return container, url, nil
}
