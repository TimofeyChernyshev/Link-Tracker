package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

type RepositorySuite struct {
	suite.Suite
	ctx context.Context

	container  testcontainers.Container
	repo       Repository
	accessType config.AccessType
	connString string
}

func TestSQLRepository(t *testing.T) {
	suite.Run(t, &RepositorySuite{accessType: config.AccessTypeSQL})
}

func TestORMRepository(t *testing.T) {
	suite.Run(t, &RepositorySuite{accessType: config.AccessTypeORM})
}

func (s *RepositorySuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		).WithDeadline(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            false,
	})
	s.Require().NoError(err)

	s.container = container

	host, _ := container.Host(s.ctx)
	port, _ := container.MappedPort(s.ctx, "5432")

	s.connString = fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	cfg := &config.Config{
		AccessType: s.accessType,
	}

	s.repo, err = NewRepository(cfg, s.connString)
	s.Require().NoError(err)

	s.ApplyMigrations()
}

func (s *RepositorySuite) TearDownSuite() {
	if s.repo != nil {
		s.repo.Close()
	}
	if s.container != nil {
		_ = s.container.Terminate(s.ctx)
	}
}

func (s *RepositorySuite) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.repo.DB().ExecContext(ctx, `
		TRUNCATE TABLE link_tags, tags, link_chat, links, chats RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		s.T().Logf("Cleanup failed: %v", err)
	}
}

func (s *RepositorySuite) ApplyMigrations() {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			id BIGINT PRIMARY KEY,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS links (
			id SERIAL PRIMARY KEY,
			url TEXT UNIQUE NOT NULL,
			updated_at TIMESTAMP DEFAULT NOW(),
			last_checked_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS link_chat (
			chat_id BIGINT NOT NULL,
			link_id INT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW(),
			PRIMARY KEY (chat_id, link_id),
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE,
			FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id SERIAL PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS link_tags (
			chat_id BIGINT NOT NULL,
			link_id INT NOT NULL,
			tag_id INT NOT NULL,
			PRIMARY KEY (chat_id, link_id, tag_id),
			FOREIGN KEY (chat_id, link_id) REFERENCES link_chat(chat_id, link_id) ON DELETE CASCADE,
			FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_links_updated_at ON links(updated_at)`,
		`CREATE INDEX IF NOT EXISTS idx_link_chat_chat_id ON link_chat(chat_id)`,
		`CREATE INDEX IF NOT EXISTS idx_link_chat_link_id ON link_chat(link_id)`,
		`CREATE INDEX IF NOT EXISTS idx_link_tags_tag_id ON link_tags(tag_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name)`,
	}

	for _, migration := range migrations {
		_, err := s.repo.DB().ExecContext(s.ctx, migration)
		s.Require().NoError(err)
	}
}

func (s *RepositorySuite) TestRegisterChat() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	exists, err := s.repo.ChatExists(s.ctx, 12345)
	s.Require().NoError(err)
	s.True(exists)
}

func (s *RepositorySuite) TestDeleteChat() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	err = s.repo.DeleteChat(s.ctx, 12345)
	s.Require().NoError(err)

	exists, err := s.repo.ChatExists(s.ctx, 12345)
	s.Require().NoError(err)
	s.False(exists)
}

func (s *RepositorySuite) TestAddLink() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	link, err := s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go", "programming"})
	s.Require().NoError(err)

	s.NotZero(link.ID)
	s.Equal("https://github.com/golang/go", link.URL)
	s.Equal([]string{"go", "programming"}, link.Tags)
	s.NotZero(link.UpdatedAt)
}

func (s *RepositorySuite) TestAddLink_AlreadyTracked() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().Error(err)
	s.Equal("link already tracked", err.Error())
}

func (s *RepositorySuite) TestRemoveLink() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	link, err := s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)

	removed, err := s.repo.RemoveLink(s.ctx, 12345, "https://github.com/golang/go")
	s.Require().NoError(err)

	s.Equal(link.ID, removed.ID)
	s.Equal(link.URL, removed.URL)

	links, err := s.repo.GetLinks(s.ctx, 12345, 10, 0)
	s.Require().NoError(err)
	s.Empty(links)
}

func (s *RepositorySuite) TestRemoveLink_NotFound() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.RemoveLink(s.ctx, 12345, "https://github.com/golang/go")
	s.Require().Error(err)
	s.Equal("link not found", err.Error())
}

func (s *RepositorySuite) TestGetLinks() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go", "programming"})
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/kubernetes/kubernetes", []string{"k8s"})
	s.Require().NoError(err)

	links, err := s.repo.GetLinks(s.ctx, 12345, 10, 0)
	s.Require().NoError(err)

	s.Len(links, 2)
	s.Equal("https://github.com/golang/go", links[0].URL)
	s.Equal([]string{"go", "programming"}, links[0].Tags)
	s.Equal("https://github.com/kubernetes/kubernetes", links[1].URL)
	s.Equal([]string{"k8s"}, links[1].Tags)
}

func (s *RepositorySuite) TestGetLinks_Pagination() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	for i := 0; i < 10; i++ {
		url := fmt.Sprintf("https://github.com/test/repo%d", i)
		_, err = s.repo.AddLink(s.ctx, 12345, url, []string{fmt.Sprintf("tag%d", i)})
		s.Require().NoError(err)
	}

	// Первая страница (limit 3)
	links, err := s.repo.GetLinks(s.ctx, 12345, 3, 0)
	s.Require().NoError(err)
	s.Len(links, 3)

	// Вторая страница
	links2, err := s.repo.GetLinks(s.ctx, 12345, 3, 3)
	s.Require().NoError(err)
	s.Len(links2, 3)

	// Проверяем, что ссылки разные
	s.NotEqual(links[0].URL, links2[0].URL)
}

func (s *RepositorySuite) TestGetLinks_WithTags() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go", "programming"})
	s.Require().NoError(err)

	links, err := s.repo.GetLinks(s.ctx, 12345, 10, 0)
	s.Require().NoError(err)

	s.Len(links, 1)
	s.Equal([]string{"go", "programming"}, links[0].Tags)
}

func (s *RepositorySuite) TestGetAllLinks() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)
	err = s.repo.RegisterChat(s.ctx, 67890)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)
	_, err = s.repo.AddLink(s.ctx, 67890, "https://github.com/kubernetes/kubernetes", []string{"k8s"})
	s.Require().NoError(err)

	s.repo.UpdateLastChecked(s.ctx, "https://github.com/golang/go", time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC))
	s.repo.UpdateLastChecked(s.ctx, "https://github.com/kubernetes/kubernetes", time.Now())

	links, err := s.repo.GetAllLinks(s.ctx, 10, 0)
	s.Require().NoError(err)

	s.Len(links, 1)
}

func (s *RepositorySuite) TestGetSubscribers() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)
	err = s.repo.RegisterChat(s.ctx, 67890)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)
	_, err = s.repo.AddLink(s.ctx, 67890, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)

	subscribers, err := s.repo.GetSubscribers(s.ctx, "https://github.com/golang/go")
	s.Require().NoError(err)

	s.Len(subscribers, 2)
	s.Contains(subscribers, int64(12345))
	s.Contains(subscribers, int64(67890))
}

func (s *RepositorySuite) TestUpdateTimestamp() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	link, err := s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)

	newTime := time.Now().Add(1 * time.Hour)
	err = s.repo.UpdateTimestamp(s.ctx, "https://github.com/golang/go", newTime)
	s.Require().NoError(err)

	links, err := s.repo.GetLinks(s.ctx, 12345, 10, 0)
	s.Require().NoError(err)
	s.True(links[0].UpdatedAt.After(link.UpdatedAt))
}

func (s *RepositorySuite) TestUpdateLastChecked() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"go"})
	s.Require().NoError(err)

	newTime := time.Now()
	err = s.repo.UpdateLastChecked(s.ctx, "https://github.com/golang/go", newTime)
	s.Require().NoError(err)
}

func (s *RepositorySuite) TestRemoveLink_ClearsUnusedTags() {
	defer s.Cleanup()

	err := s.repo.RegisterChat(s.ctx, 12345)
	s.Require().NoError(err)

	_, err = s.repo.AddLink(s.ctx, 12345, "https://github.com/golang/go", []string{"unique-tag"})
	s.Require().NoError(err)

	var count int
	err = s.repo.DB().QueryRowContext(s.ctx, `SELECT COUNT(*) FROM tags WHERE name = 'unique-tag'`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)

	_, err = s.repo.RemoveLink(s.ctx, 12345, "https://github.com/golang/go")
	s.Require().NoError(err)

	err = s.repo.DB().QueryRowContext(s.ctx, `SELECT COUNT(*) FROM tags WHERE name = 'unique-tag'`).Scan(&count)
	s.Require().NoError(err)
	s.Equal(0, count)
}
