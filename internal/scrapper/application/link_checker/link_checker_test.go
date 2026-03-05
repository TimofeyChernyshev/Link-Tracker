package linkchecker

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkCheckerSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockClient1  *MockClient
	mockClient2  *MockClient
	mockNotifier *MockNotifier
	mockStorage  *MockStorage
	checker      *LinkChecker
	ctx          context.Context
}

func (s *LinkCheckerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockClient1 = NewMockClient(s.ctrl)
	s.mockClient2 = NewMockClient(s.ctrl)
	s.mockNotifier = NewMockNotifier(s.ctrl)
	s.mockStorage = NewMockStorage(s.ctrl)

	s.checker = New(
		[]Client{s.mockClient1, s.mockClient2},
		s.mockNotifier,
		s.mockStorage,
	)
	s.ctx = context.Background()
}

func (s *LinkCheckerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestLinkCheckerSuite(t *testing.T) {
	suite.Run(t, new(LinkCheckerSuite))
}

func (s *LinkCheckerSuite) TestCheckUpdates_NoLinks() {
	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{})

	s.checker.CheckUpdates(s.ctx)
}

func (s *LinkCheckerSuite) TestCheckUpdates_SingleLink_NoChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient1.EXPECT().Check(s.ctx, link).Return(false, "", nil)
	s.mockClient2.EXPECT().Check(s.ctx, link).Return(false, "", nil)

	s.checker.CheckUpdates(s.ctx)
}

func (s *LinkCheckerSuite) TestCheckUpdates_SingleLink_WithChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient1.EXPECT().Check(s.ctx, link).Return(false, "", nil)

	description := "New commit added"
	s.mockClient2.EXPECT().Check(s.ctx, link).Return(true, description, nil)

	s.mockStorage.EXPECT().UpdateTimestamp(link.URL, gomock.Any()).Do(
		func(url string, ts time.Time) {
			assert.WithinDuration(s.T(), time.Now(), ts, time.Second)
		},
	)

	chatIds := []int64{123, 456}
	s.mockStorage.EXPECT().GetSubscribers(link.URL).Return(chatIds)

	expectedUpdate := domain.LinkUpdate{
		ID:          link.ID,
		URL:         link.URL,
		Description: description,
		ChatIds:     chatIds,
	}
	s.mockNotifier.EXPECT().SendUpdate(s.ctx, expectedUpdate).Return(nil)

	s.checker.CheckUpdates(s.ctx)
}

func (s *LinkCheckerSuite) TestCheckUpdates_MultipleClientsForSameLink() {
	link := domain.Link{
		ID:  1,
		URL: "https://api.github.com/repos/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient1.EXPECT().Check(s.ctx, link).Return(false, "", nil)
	s.mockClient2.EXPECT().Check(s.ctx, link).Return(true, "update from second", nil)

	s.mockStorage.EXPECT().UpdateTimestamp(link.URL, gomock.Any()).Times(1)
	s.mockStorage.EXPECT().GetSubscribers(link.URL).Return([]int64{1})
	s.mockNotifier.EXPECT().SendUpdate(s.ctx, gomock.Any()).Return(nil)

	s.checker.CheckUpdates(s.ctx)
}
