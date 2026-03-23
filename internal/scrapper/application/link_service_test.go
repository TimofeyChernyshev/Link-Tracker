package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ServiceSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockClient   *MockClient
	mockNotifier *MockNotifier
	mockStorage  *MockStorage
	service      *Service
	ctx          context.Context
}

func (s *ServiceSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockClient = NewMockClient(s.ctrl)
	s.mockNotifier = NewMockNotifier(s.ctrl)
	s.mockStorage = NewMockStorage(s.ctrl)

	s.service = NewLinkService(
		s.mockClient,
		s.mockNotifier,
		s.mockStorage,
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
	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{})

	s.service.CheckUpdates(s.ctx)
}

func (s *ServiceSuite) TestCheckUpdates_GotError() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient.EXPECT().Check(s.ctx, link).Return(false, "", errors.New("some text"))

	s.service.CheckUpdates(s.ctx)
}

func (s *ServiceSuite) TestCheckUpdates_NoChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient.EXPECT().Check(s.ctx, link).Return(false, "", nil)

	s.service.CheckUpdates(s.ctx)
}

func (s *ServiceSuite) TestCheckUpdates_WithChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	description := "New commit added"
	s.mockClient.EXPECT().Check(s.ctx, link).Return(true, description, nil)

	s.mockStorage.EXPECT().UpdateTimestamp(link.URL, gomock.Any()).Do(
		func(_ string, ts time.Time) {
			s.WithinDuration(time.Now(), ts, time.Second)
		},
	)

	chatIDs := []int64{123, 456}
	s.mockStorage.EXPECT().GetSubscribers(link.URL).Return(chatIDs)

	expectedUpdate := domain.LinkUpdate{
		ID:          link.ID,
		URL:         link.URL,
		Description: description,
		ChatIDs:     chatIDs,
	}
	s.mockNotifier.EXPECT().SendUpdate(s.ctx, expectedUpdate).Return(nil)

	s.service.CheckUpdates(s.ctx)
}

func (s *ServiceSuite) TestAddLink_Success() {
	chatID := int64(1)
	url := "https://example.com"
	tags := []string{"go"}

	expected := domain.Link{
		URL:  url,
		Tags: tags,
	}

	s.mockStorage.EXPECT().ChatExists(chatID).Return(true)
	s.mockStorage.EXPECT().AddLink(chatID, url, tags).Return(expected, nil)

	link, err := s.service.AddLink(chatID, url, tags)

	s.Require().NoError(err)
	s.Equal(expected, link)
}

func (s *ServiceSuite) TestAddLink_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(false)

	link, err := s.service.AddLink(1, "url", nil)

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
	s.Equal(domain.Link{}, link)
}

func (s *ServiceSuite) TestRemoveLink_Success() {
	expected := domain.Link{URL: "https://example.com"}

	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(true)
	s.mockStorage.EXPECT().RemoveLink(int64(1), expected.URL).Return(expected, nil)

	link, err := s.service.RemoveLink(1, expected.URL)

	s.Require().NoError(err)
	s.Equal(expected, link)
}

func (s *ServiceSuite) TestRemoveLink_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(false)

	_, err := s.service.RemoveLink(1, "url")

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
}

func (s *ServiceSuite) TestGetLinks_Success() {
	expected := []domain.Link{
		{URL: "a"},
		{URL: "b"},
	}

	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(true)
	s.mockStorage.EXPECT().GetLinks(int64(1)).Return(expected)

	links, err := s.service.GetLinks(1)

	s.Require().NoError(err)
	s.Equal(expected, links)
}

func (s *ServiceSuite) TestGetLinks_ChatNotExists() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(false)

	links, err := s.service.GetLinks(1)

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
	s.Nil(links)
}

func (s *ServiceSuite) TestRegisterChat_Success() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(false)
	s.mockStorage.EXPECT().RegisterChat(int64(1))

	err := s.service.RegisterChat(1)

	s.Require().NoError(err)
}

func (s *ServiceSuite) TestRegisterChat_AlreadyExists() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(true)

	err := s.service.RegisterChat(1)

	s.Require().Error(err)
	s.Equal("chat already registered", err.Error())
}

func (s *ServiceSuite) TestDeleteChat_Success() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(true)
	s.mockStorage.EXPECT().DeleteChat(int64(1))

	err := s.service.DeleteChat(1)

	s.Require().NoError(err)
}

func (s *ServiceSuite) TestDeleteChat_NotExists() {
	s.mockStorage.EXPECT().ChatExists(int64(1)).Return(false)

	err := s.service.DeleteChat(1)

	s.Require().Error(err)
	s.ErrorIs(err, errChatInstRegistered)
}
