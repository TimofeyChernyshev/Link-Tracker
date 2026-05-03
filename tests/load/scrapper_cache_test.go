package benchmark

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
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

var (
	getRequests  = 100
	postRequests = 1
	users        = 1000
	linksPerUser = 100

	linkServiceLimit = 100

	testDuration = 5 * time.Minute
	rampUp       = time.Minute
)

type Metrics struct {
	mu sync.Mutex

	getLatencies  []time.Duration
	postLatencies []time.Duration

	getSuccess  int
	postSuccess int

	postErr500 int
	getErr500  int
}

func TestLoad(t *testing.T) {
	ctx := context.Background()

	postgresContainer, connString, err := startPostgresContainer(ctx)
	require.NoError(t, err)
	defer postgresContainer.Terminate(ctx)

	err = runMigrations(connString)
	require.NoError(t, err)

	repo, err := sqlrepo.NewRepository(connString)
	require.NoError(t, err)
	defer repo.Close()

	valkeyContainer, valkeyAddr, err := startValkeySingleNode(ctx)
	require.NoError(t, err)
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
	require.NoError(t, err)

	serviceNoCache := application.NewLinkService(nil, nil, repo, &NoopCache{}, 100, 1, linkServiceLimit)
	serviceCache := application.NewLinkService(nil, nil, repo, valkeyClient, 100, 1, linkServiceLimit)

	for u := range users {
		chatID := int64(u)

		err = repo.RegisterChat(ctx, chatID)
		require.NoError(t, err)

		for i := range linksPerUser {
			_, err = repo.AddLink(ctx, chatID, "https://"+strconv.Itoa(i), []string{"tag"})
			require.NoError(t, err)
		}
	}

	t.Run("no_cache", func(_ *testing.T) {
		RunLoadTest(serviceNoCache)
	})

	t.Run("with_cache", func(_ *testing.T) {
		RunLoadTest(serviceCache)
	})
}

func RunLoadTest(svc *application.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	workers := runtime.NumCPU() * 2

	metrics := &Metrics{
		getLatencies:  make([]time.Duration, 0, 100000),
		postLatencies: make([]time.Duration, 0, 100000),
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(i int) {
			defer wg.Done()

			delay := time.Duration(i) * rampUp / time.Duration(workers)
			time.Sleep(delay)

			worker(ctx, svc, metrics)
		}(i)
	}

	<-ctx.Done()
	wg.Wait()

	printStats("GET /list", metrics.getLatencies, metrics.getSuccess, metrics.getErr500)
	printStats("POST /list", metrics.postLatencies, metrics.postSuccess, metrics.postErr500)
}

func worker(ctx context.Context, svc *application.Service, m *Metrics) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for range getRequests {
			chatID := int64(rnd.Intn(users))

			start := time.Now()
			_, err := svc.GetLinks(ctx, chatID, linkServiceLimit, 0)
			duration := time.Since(start)

			m.mu.Lock()
			m.getLatencies = append(m.getLatencies, duration)
			if err != nil {
				m.getErr500++
			} else {
				m.getSuccess++
			}
			m.mu.Unlock()
		}

		for range postRequests {
			chatID := int64(rnd.Intn(users))

			start := time.Now()
			_, err := svc.AddLink(ctx, chatID, "https://test", nil)
			duration := time.Since(start)

			m.mu.Lock()
			m.postLatencies = append(m.postLatencies, duration)
			if err != nil {
				m.postErr500++
			} else {
				m.postSuccess++
			}
			m.mu.Unlock()
		}
	}
}

func printStats(name string, latencies []time.Duration, success, err500 int) {
	if len(latencies) == 0 {
		fmt.Println(name, "no data")
		return
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	total := len(latencies)

	p50 := latencies[total/2]
	p99 := latencies[int(float64(total)*0.99)]

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}

	avg := sum / time.Duration(total)
	rps := float64(total) / testDuration.Seconds()

	fmt.Println(name)
	fmt.Println("Total:", total)
	fmt.Println("Success:", success)
	fmt.Println("Errors 500:", err500)
	fmt.Println("Avg:", avg)
	fmt.Println("P50:", p50)
	fmt.Println("P99:", p99)
	fmt.Println("RPS:", rps)
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
