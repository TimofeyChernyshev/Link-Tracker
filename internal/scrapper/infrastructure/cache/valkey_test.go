package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ValkeyClientTestSuite struct {
	suite.Suite
	client          *ValkeyClient
	ctx             context.Context
	valkeyContainer testcontainers.Container
}

func TestValkeyClientSuite(t *testing.T) {
	suite.Run(t, new(ValkeyClientTestSuite))
}

func (s *ValkeyClientTestSuite) SetupSuite() {
	s.ctx = context.Background()

	testcontainers.SkipIfProviderIsNotHealthy(s.T())

	req := testcontainers.ContainerRequest{
		Image:        "valkey/valkey:8.0-alpine",
		ExposedPorts: []string{"6379/tcp"},
		Cmd: []string{
			"valkey-server",
			"--port", "6379",
		},
		WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)

	s.valkeyContainer = container

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	port, err := container.MappedPort(s.ctx, "6379")
	s.Require().NoError(err)

	address := fmt.Sprintf("%s:%s", host, port.Port())

	redisClient := redis.NewClient(&redis.Options{
		Addr: address,
	})

	err = redisClient.Ping(s.ctx).Err()
	s.Require().NoError(err)

	s.client = &ValkeyClient{
		client: redisClient,
		ttl:    5 * time.Minute,
	}
}

func (s *ValkeyClientTestSuite) TearDownTest() {
	_ = s.client.InvalidateLinks(s.ctx, 12345)
}

func (s *ValkeyClientTestSuite) TearDownSuite() {
	if s.client != nil {
		err := s.client.Close()
		s.Require().NoError(err)
	}

	if s.valkeyContainer != nil {
		err := s.valkeyContainer.Terminate(s.ctx)
		s.Require().NoError(err)
	}
}

func (s *ValkeyClientTestSuite) TestSetAndGetLinks() {
	chatID := int64(12345)

	testLinks := []domain.Link{
		{
			ID:        1,
			URL:       "https://234",
			Tags:      []string{"2", "3"},
			UpdatedAt: time.Now().Truncate(time.Second),
		},
		{
			ID:        2,
			URL:       "https://123",
			Tags:      []string{"4", "5"},
			UpdatedAt: time.Now().Truncate(time.Second),
		},
	}

	err := s.client.SetLinks(s.ctx, chatID, testLinks)
	s.Require().NoError(err)

	cachedLinks, err := s.client.GetLinks(s.ctx, chatID)
	s.Require().NoError(err)

	s.Len(cachedLinks, 2)
	s.Equal(testLinks[0].ID, cachedLinks[0].ID)
	s.Equal(testLinks[0].URL, cachedLinks[0].URL)
	s.Equal(testLinks[0].Tags, cachedLinks[0].Tags)

	s.Equal(testLinks[1].ID, cachedLinks[1].ID)
	s.Equal(testLinks[1].URL, cachedLinks[1].URL)
	s.Equal(testLinks[1].Tags, cachedLinks[1].Tags)
}

func (s *ValkeyClientTestSuite) TestGetLinks_Empty() {
	chatID := int64(99999)

	links, err := s.client.GetLinks(s.ctx, chatID)
	s.Require().NoError(err)
	s.Empty(links)
}

func (s *ValkeyClientTestSuite) TestGetLinks_NonExistentKey() {
	chatID1 := int64(11111123123)
	chatID2 := int64(222222121)

	testLinks := []domain.Link{
		{ID: 1, URL: "https://test.com", Tags: []string{"test"}, UpdatedAt: time.Now()},
	}

	err := s.client.SetLinks(s.ctx, chatID1, testLinks)
	s.Require().NoError(err)

	links, err := s.client.GetLinks(s.ctx, chatID2)
	s.Require().NoError(err)
	s.Empty(links)

	links, err = s.client.GetLinks(s.ctx, chatID1)
	s.Require().NoError(err)
	s.Len(links, 1)
}

func (s *ValkeyClientTestSuite) TestInvalidateLinks() {
	chatID := int64(12345)

	testLinks := []domain.Link{
		{ID: 1, URL: "https://test.com", Tags: []string{"test"}, UpdatedAt: time.Now()},
	}

	err := s.client.SetLinks(s.ctx, chatID, testLinks)
	s.Require().NoError(err)

	links, err := s.client.GetLinks(s.ctx, chatID)
	s.Require().NoError(err)
	s.Len(links, 1)

	err = s.client.InvalidateLinks(s.ctx, chatID)
	s.Require().NoError(err)

	links, err = s.client.GetLinks(s.ctx, chatID)
	s.Require().NoError(err)
	s.Empty(links)
}

func (s *ValkeyClientTestSuite) TestInvalidateLinks_NonExistent() {
	chatID := int64(99999)

	err := s.client.InvalidateLinks(s.ctx, chatID)
	s.NoError(err)
}

func (s *ValkeyClientTestSuite) TestOverwrite() {
	chatID := int64(12345)

	firstLinks := []domain.Link{
		{ID: 1, URL: "https://first.com", Tags: []string{"first"}, UpdatedAt: time.Now()},
	}

	err := s.client.SetLinks(s.ctx, chatID, firstLinks)
	s.Require().NoError(err)

	secondLinks := []domain.Link{
		{ID: 2, URL: "https://second.com", Tags: []string{"second"}, UpdatedAt: time.Now()},
		{ID: 3, URL: "https://third.com", Tags: []string{"third"}, UpdatedAt: time.Now()},
	}

	err = s.client.SetLinks(s.ctx, chatID, secondLinks)
	s.Require().NoError(err)

	cachedLinks, err := s.client.GetLinks(s.ctx, chatID)
	s.Require().NoError(err)

	s.Len(cachedLinks, 2)
	s.Equal(int64(2), cachedLinks[0].ID)
	s.Equal("https://second.com", cachedLinks[0].URL)
	s.Equal(int64(3), cachedLinks[1].ID)
	s.Equal("https://third.com", cachedLinks[1].URL)
}

func (s *ValkeyClientTestSuite) TestDifferentChatIDs() {
	chats := []int64{11111, 22222, 33333}
	linksPerChat := make(map[int64][]domain.Link)

	for i, chatID := range chats {
		links := []domain.Link{
			{
				ID:        int64(i + 1),
				URL:       fmt.Sprintf("https://chat%d.com", chatID),
				Tags:      []string{fmt.Sprintf("tag%d", i)},
				UpdatedAt: time.Now(),
			},
		}
		linksPerChat[chatID] = links

		err := s.client.SetLinks(s.ctx, chatID, links)
		s.Require().NoError(err)
	}

	for chatID, expectedLinks := range linksPerChat {
		cachedLinks, err := s.client.GetLinks(s.ctx, chatID)
		s.Require().NoError(err)
		s.Len(cachedLinks, 1)
		s.Equal(expectedLinks[0].URL, cachedLinks[0].URL)
	}
}
