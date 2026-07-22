package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	testBatchSize    = 10
	testWorkerCount  = 2
	testInterval     = time.Duration(100000)
	testDefaultLimit = 20
)

type ServiceSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockClient   *MockClient
	mockNotifier *MockNotifier
	mockStorage  *MockStorage
	mockCache    *MockCache
	service      *Service
	ctx          context.Context
}

func (s *ServiceSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockClient = NewMockClient(s.ctrl)
	s.mockNotifier = NewMockNotifier(s.ctrl)
	s.mockStorage = NewMockStorage(s.ctrl)
	s.mockCache = NewMockCache(s.ctrl)

	s.service = NewLinkService(
		s.mockClient,
		s.mockNotifier,
		s.mockStorage,
		s.mockCache,
		nil,
		testBatchSize,
		testWorkerCount,
		testDefaultLimit,
	)
	s.ctx = context.Background()
}

func (s *ServiceSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}

func (s *ServiceSuite) TestCheckUpdates_NoLinks() {
	s.mockStorage.EXPECT().GetLinksWithInterval(gomock.Any(), testBatchSize, defaultOffset, testInterval).Return([]domain.Link{}, nil)

	s.service.CheckUpdates(s.ctx, testInterval)
}

func (s *ServiceSuite) TestCheckUpdates_GotError() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 0, testInterval).Return([]domain.Link{link}, nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, testBatchSize, testInterval).Return([]domain.Link{}, nil)

	s.mockClient.EXPECT().Check(s.ctx, link).Return(nil, errors.New("some text"))

	s.mockStorage.EXPECT().GetSubscribers(s.ctx, link.URL).Return([]int64{1}, nil)

	s.mockNotifier.EXPECT().SendUpdate(s.ctx, domain.LinkUpdate{
		Description: "Ошибки при проверке ссылок\n" +
			"Не удалось обработать 1 ссылок:\n- https://github.com/user/repo: check failed: some text",
		ChatIDs: []int64{1},
	}).Return(nil)

	s.service.CheckUpdates(s.ctx, testInterval)
}

func (s *ServiceSuite) TestCheckUpdates_NoChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 0, testInterval).Return([]domain.Link{link}, nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, testBatchSize, testInterval).Return([]domain.Link{}, nil)

	s.mockClient.EXPECT().Check(s.ctx, link).Return(nil, nil)

	s.mockStorage.EXPECT().UpdateLastChecked(s.ctx, link.URL, gomock.Any()).Return(nil)

	s.service.CheckUpdates(s.ctx, testInterval)
}

func (s *ServiceSuite) TestCheckUpdates_WithChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 0, testInterval).Return([]domain.Link{link}, nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, testBatchSize, testInterval).Return([]domain.Link{}, nil)

	description := "New commit added"
	s.mockClient.EXPECT().Check(s.ctx, link).Return([]domain.Event{{Description: description, OccurredAt: time.Now()}}, nil)

	s.mockStorage.EXPECT().UpdateLastChecked(s.ctx, "https://github.com/user/repo", gomock.Any()).Return(nil)

	s.mockStorage.EXPECT().UpdateTimestamp(s.ctx, link.URL, gomock.Any()).Do(
		func(_ context.Context, _ string, ts time.Time) {
			s.WithinDuration(time.Now(), ts, time.Second)
		},
	).Return(nil)

	chatIDs := []int64{123, 456}
	s.mockStorage.EXPECT().GetSubscribers(s.ctx, link.URL).Return(chatIDs, nil)

	expectedUpdate := domain.LinkUpdate{
		ID:          link.ID,
		URL:         link.URL,
		Description: description,
		ChatIDs:     chatIDs,
	}
	s.mockNotifier.EXPECT().SendUpdate(s.ctx, expectedUpdate).Return(nil)

	s.service.CheckUpdates(s.ctx, testInterval)
}

func (s *ServiceSuite) TestCheckUpdates_Pagination() {
	linksLen := 25
	links := make([]domain.Link, linksLen)
	for i := range linksLen {
		links[i] = domain.Link{
			ID:  int64(i + 1),
			URL: fmt.Sprintf("https://github.com/repo%d", i+1),
		}
	}

	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 0, testInterval).Return(links[0:10], nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 10, testInterval).Return(links[10:20], nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 20, testInterval).Return(links[20:25], nil)
	s.mockStorage.EXPECT().GetLinksWithInterval(s.ctx, testBatchSize, 30, testInterval).Return([]domain.Link{}, nil)

	for i := range linksLen {
		s.mockClient.EXPECT().Check(s.ctx, links[i]).Return(nil, nil)
		s.mockStorage.EXPECT().UpdateLastChecked(s.ctx, links[i].URL, gomock.Any()).Return(nil)
	}

	s.service.CheckUpdates(s.ctx, testInterval)
}

func (s *ServiceSuite) TestAddLink_Success() {
	chatID := int64(1)
	url := "https://example.com"
	tags := []string{"go"}

	expected := domain.Link{
		URL:  url,
		Tags: tags,
	}

	s.mockStorage.EXPECT().ChatExists(s.ctx, chatID).Return(true, nil)
	s.mockStorage.EXPECT().IsSubscribed(s.ctx, chatID, url).Return(false, nil)
	s.mockStorage.EXPECT().AddLink(s.ctx, chatID, url, tags).Return(expected, nil)
	s.mockCache.EXPECT().InvalidateLinks(s.ctx, chatID).Return(nil)

	link, err := s.service.AddLink(s.ctx, chatID, url, tags)

	s.Require().NoError(err)
	s.Equal(expected, link)
}

func (s *ServiceSuite) TestAddLink_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(false, nil)

	link, err := s.service.AddLink(s.ctx, 1, "url", nil)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errChatInstRegistered)
	s.Equal(domain.Link{}, link)
}

func (s *ServiceSuite) TestAddLink_AlreadyExist() {
	chatID := int64(1)
	url := "https://example.com"
	tags := []string{"go"}

	s.mockStorage.EXPECT().ChatExists(s.ctx, chatID).Return(true, nil)
	s.mockStorage.EXPECT().IsSubscribed(s.ctx, chatID, url).Return(true, nil)

	_, err := s.service.AddLink(s.ctx, chatID, url, tags)

	s.Require().ErrorContains(err, "link already tracked")
}

func (s *ServiceSuite) TestRemoveLink_Success() {
	expected := domain.Link{URL: "https://example.com"}

	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(true, nil)
	s.mockStorage.EXPECT().RemoveLink(s.ctx, int64(1), expected.URL).Return(expected, nil)
	s.mockCache.EXPECT().InvalidateLinks(s.ctx, int64(1)).Return(nil)

	link, err := s.service.RemoveLink(s.ctx, 1, expected.URL)

	s.Require().NoError(err)
	s.Equal(expected, link)
}

func (s *ServiceSuite) TestRemoveLink_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(false, nil)

	_, err := s.service.RemoveLink(s.ctx, 1, "url")

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
}

func (s *ServiceSuite) TestGetLinks_Success() {
	expected := []domain.Link{
		{URL: "a"},
		{URL: "b"},
	}

	limit := 10
	offset := 0
	s.service.defaultLimit = limit

	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(true, nil)
	s.mockCache.EXPECT().GetLinks(s.ctx, int64(1), limit, offset).Return(nil, nil) // cache miss
	s.mockStorage.EXPECT().GetLinks(s.ctx, int64(1), limit, offset).Return(expected, nil)
	s.mockCache.EXPECT().SetLinks(s.ctx, int64(1), limit, offset, expected).Return(nil)

	links, err := s.service.GetLinks(s.ctx, 1, limit, offset)

	s.Require().NoError(err)
	s.Equal(expected, links)
}

func (s *ServiceSuite) TestGetLinks_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(false, nil)

	links, err := s.service.GetLinks(s.ctx, 1, 10, 0)

	s.Require().Error(err)
	s.Require().ErrorIs(err, errChatInstRegistered)
	s.Nil(links)
}

func (s *ServiceSuite) TestGetLinks_CacheHit() {
	expected := []domain.Link{
		{URL: "a"},
		{URL: "b"},
	}

	limit := 10
	offset := 0
	s.service.defaultLimit = limit

	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(true, nil)
	s.mockCache.EXPECT().GetLinks(s.ctx, int64(1), limit, offset).Return(expected, nil)

	links, err := s.service.GetLinks(s.ctx, 1, limit, offset)

	s.Require().NoError(err)
	s.Equal(expected, links)
}

func (s *ServiceSuite) TestRegisterChat_Success() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(false, nil)
	s.mockStorage.EXPECT().RegisterChat(s.ctx, int64(1)).Return(nil)

	err := s.service.RegisterChat(s.ctx, 1)

	s.Require().NoError(err)
}

func (s *ServiceSuite) TestRegisterChat_AlreadyExists() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(true, nil)

	err := s.service.RegisterChat(s.ctx, 1)

	s.Require().Error(err)
	s.Equal("chat already registered", err.Error())
}

func (s *ServiceSuite) TestDeleteChat_Success() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(true, nil)
	s.mockStorage.EXPECT().DeleteChat(s.ctx, int64(1)).Return(nil)
	s.mockCache.EXPECT().InvalidateLinks(s.ctx, int64(1)).Return(nil)

	err := s.service.DeleteChat(s.ctx, 1)

	s.Require().NoError(err)
}

func (s *ServiceSuite) TestDeleteChat_NotExists() {
	s.mockStorage.EXPECT().ChatExists(s.ctx, int64(1)).Return(false, nil)

	err := s.service.DeleteChat(s.ctx, 1)

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
}
