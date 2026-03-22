package linkchecker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkCheckerSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockClient   *MockClient
	mockNotifier *MockNotifier
	mockStorage  *MockStorage
	checker      *LinkChecker
	ctx          context.Context
}

func (s *LinkCheckerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockClient = NewMockClient(s.ctrl)
	s.mockNotifier = NewMockNotifier(s.ctrl)
	s.mockStorage = NewMockStorage(s.ctrl)

	s.checker = New(
		s.mockClient,
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

func (s *LinkCheckerSuite) TestCheckUpdates_GotError() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient.EXPECT().Check(s.ctx, link).Return(false, "", fmt.Errorf("some text"))

	s.checker.CheckUpdates(s.ctx)
}

func (s *LinkCheckerSuite) TestCheckUpdates_NoChanges() {
	link := domain.Link{
		ID:  1,
		URL: "https://github.com/user/repo",
	}

	s.mockStorage.EXPECT().GetAllLinks().Return([]domain.Link{link})

	s.mockClient.EXPECT().Check(s.ctx, link).Return(false, "", nil)

	s.checker.CheckUpdates(s.ctx)
}

func (s *LinkCheckerSuite) TestCheckUpdates_WithChanges() {
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

	s.checker.CheckUpdates(s.ctx)
}
