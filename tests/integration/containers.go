package integration

import (
	"context"
	"io"
	"os"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartScrapper(ctx context.Context, networkName string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image: "linktracker-scrapper",

		ExposedPorts: []string{"8081/tcp"},

		Env: map[string]string{
			"PORT":         "8081",
			"BOT_BASE_URL": "http://bot:8080",
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
		logs, _ := container.Logs(ctx)
		io.Copy(os.Stdout, logs)
		return nil, "", err
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8081/tcp")

	url := "http://" + host + ":" + mappedPort.Port()

	return container, url, nil
}

func StartBot(ctx context.Context, networkName string, telegramURL string) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image: "linktracker-bot",

		ExposedPorts: []string{"8080/tcp"},

		Env: map[string]string{
			"TELEGRAM_TOKEN":    "test",
			"TELEGRAM_API_URL":  telegramURL,
			"PORT":              "8080",
			"SCRAPPER_BASE_URL": "http://scrapper:8081",
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
		logs, _ := container.Logs(ctx)
		io.Copy(os.Stdout, logs)
		return nil, "", err
	}

	host, _ := container.Host(ctx)
	mappedPort, _ := container.MappedPort(ctx, "8080/tcp")

	url := "http://" + host + ":" + mappedPort.Port()

	return container, url, nil
}
