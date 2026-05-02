package benchmark

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // для миграций
	_ "github.com/golang-migrate/migrate/v4/source/file"       // для миграций
	_ "github.com/jackc/pgx/v5/stdlib"                         // для запуска Postgres
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/cache"
	sqlrepo "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/repository/sql"
)

func BenchmarkGetLinks_CacheVsNoCache(b *testing.B) {
	ctx := context.Background()

	postgresContainer, connString, err := startPostgresContainer(ctx)
	require.NoError(b, err)
	defer postgresContainer.Terminate(ctx)

	err = runMigrations(connString)
	require.NoError(b, err)

	repo, err := sqlrepo.NewRepository(connString)
	require.NoError(b, err)
	defer repo.Close()

	valkeyContainer, valkeyAddr, err := startValkeySingleNode(ctx)
	require.NoError(b, err)
	defer valkeyContainer.Terminate(ctx)

	valkeyClient, err := cache.NewValkeyClient(
		[]string{valkeyAddr},
		"",
		time.Minute,
		10,
		3,
		time.Millisecond*100,
		time.Second,
		time.Second,
		false,
	)
	require.NoError(b, err)

	serviceNoCache := application.NewLinkService(nil, nil, repo, &NoopCache{}, 100, 1, 100)
	serviceCache := application.NewLinkService(nil, nil, repo, valkeyClient, 100, 1, 100)

	chatID := int64(1)
	err = repo.RegisterChat(ctx, chatID)
	require.NoError(b, err)

	for i := 0; i < 1000; i++ {
		_, err := serviceNoCache.AddLink(
			ctx,
			chatID,
			"https://example.com/"+strconv.Itoa(i),
			[]string{"tag1", "tag2"},
		)
		require.NoError(b, err)
	}

	b.Run("no_cache", func(b *testing.B) {
		for range b.N {
			_, err := serviceNoCache.GetLinks(ctx, chatID, 100, 0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with_cache", func(b *testing.B) {
		// сохранение результата операции в кэш
		_, err := serviceCache.GetLinks(ctx, chatID, 100, 0)
		require.NoError(b, err)

		b.ResetTimer()

		for range b.N {
			_, err := serviceCache.GetLinks(ctx, chatID, 100, 0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func startPostgresContainer(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started:          true,
		ContainerRequest: req,
	})
	if err != nil {
		return nil, "", fmt.Errorf("cannot create postgres container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")

	connString := fmt.Sprintf(
		"postgres://postgres:postgres@%s:%s/test?sslmode=disable",
		host,
		port.Port(),
	)

	return container, connString, nil
}

func runMigrations(connString string) error {
	_, filename, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(filename)

	root := filepath.Join(baseDir, "..", "..")

	migrationsPath := "file://" + filepath.ToSlash(filepath.Join(root, "migrations"))

	m, err := migrate.New(
		migrationsPath,
		connString,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func startValkeySingleNode(ctx context.Context) (testcontainers.Container, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "valkey/valkey:8.0-alpine",
		ExposedPorts: []string{"6379/tcp"},
		Cmd: []string{
			"valkey-server",
			"--port", "6379",
		},
		WaitingFor: wait.ForListeningPort("6379/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", err
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "6379")

	addr := host + ":" + port.Port()

	return container, addr, nil
}
