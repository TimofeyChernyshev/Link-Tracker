package integration

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

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

			"VALKEY_ADDRESSES":    "valkey-node-1:6379,valkey-node-2:6379,valkey-node-3:6379",
			"VALKEY_CLUSTER_MODE": "true",
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

func StartValkeyNode(ctx context.Context, networkName string) ([]testcontainers.Container, error) {
	nodesInCluster := 3
	nodes := make([]testcontainers.Container, 0, nodesInCluster)

	for i := 1; i <= nodesInCluster; i++ {
		name := fmt.Sprintf("valkey-node-%d", i)

		req := testcontainers.ContainerRequest{
			Image: "valkey/valkey:8.0-alpine",
			Cmd: []string{
				"valkey-server",
				"--cluster-enabled", "yes",
				"--cluster-config-file", fmt.Sprintf("/data/nodes-%d.conf", i),
				"--cluster-node-timeout", "5000",
				"--appendonly", "yes",
				"--appendfsync", "everysec",
				"--port", "6379",
				"--cluster-announce-port", "6379",
				"--cluster-announce-bus-port", "16379",
				"--cluster-announce-ip", name,
				"--maxmemory", "256mb",
				"--maxmemory-policy", "allkeys-lru",
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {name},
			},
			WaitingFor: wait.ForExec([]string{"valkey-cli", "ping"}).
				WithExitCodeMatcher(func(code int) bool { return code == 0 }),
		}

		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			return nil, fmt.Errorf("starting valkey node: %w", err)
		}

		nodes = append(nodes, c)
	}

	return nodes, nil
}

func InitValkeyCluster(ctx context.Context, networkName string) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image: "valkey/valkey:8.0-alpine",
		Cmd: []string{
			"sh", "-c",
			`
			valkey-cli --cluster create \
			  valkey-node-1:6379 \
			  valkey-node-2:6379 \
			  valkey-node-3:6379 \
			  --cluster-replicas 0 \
			  --cluster-yes

			echo "Cluster ready"
			`,
		},
		Networks:   []string{networkName},
		WaitingFor: wait.ForLog("Cluster ready"),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("initialization of valkey cluster: %w", err)
	}

	return c, nil
}

func IsValkeyClusterReady(ctx context.Context, networkName string) bool {
	req := testcontainers.ContainerRequest{
		Image: "valkey/valkey:8.0-alpine",
		Cmd: []string{
			"valkey-cli",
			"-h", "valkey-node-1",
			"cluster", "info",
		},
		Networks: []string{networkName},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return false
	}
	defer func() {
		_ = c.Terminate(ctx)
	}()

	logs, err := c.Logs(ctx)
	if err != nil {
		return false
	}
	defer func() {
		_ = logs.Close()
	}()

	buf := new(strings.Builder)
	_, _ = io.Copy(buf, logs)

	return strings.Contains(buf.String(), "cluster_state:ok")
}

func StartPostgres(ctx context.Context, networkName string) (testcontainers.Container, error) {
	occurrence := 2 // 1 - БД инициализируется, 2 - БД готова к подключению
	startupTimeout := 60 * time.Second

	postgresReq := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "linktracker",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"postgres"},
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(occurrence).
			WithStartupTimeout(startupTimeout),
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: postgresReq,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("starting postgres: %w", err)
	}

	return postgresContainer, nil
}
