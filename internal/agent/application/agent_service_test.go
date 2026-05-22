package application

import (
	"context"
	"errors"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

const (
	testFilterMinLenght        = 10
	testsummarizationThreshold = 20
	testGroupingWindow         = 100 * time.Millisecond
	sendTimeout                = 30 * time.Second
)

type ServiceSuite struct {
	suite.Suite
	ctrl         *gomock.Controller
	mockNotifier *MockNotifier
	service      *AgentService
	ctx          context.Context
}

func (s *ServiceSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockNotifier = NewMockNotifier(s.ctrl)

	s.service = NewAgentService(
		[]string{"stop", "word"},
		[]string{"author1"},
		[]string{"high"}, []string{"low"},
		testFilterMinLenght,
		testsummarizationThreshold,
		testGroupingWindow,
		sendTimeout,
		s.mockNotifier,
	)
	s.ctx = context.Background()
}

func (s *ServiceSuite) TearDownTest() {
	s.ctrl.Finish()
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}

func (s *ServiceSuite) TestFilter() {
	tests := []struct {
		name                    string
		upd                     domain.RawUpdate
		expectedProcessedUpdate domain.ProcessedUpdate
		expectedIsFiltered      bool
	}{
		{
			name: "min length filtration",
			upd: domain.RawUpdate{
				ID:          1,
				Description: "123",
				Author:      "1",
				TgChatIDs:   []int64{1, 2},
			},
			expectedProcessedUpdate: domain.ProcessedUpdate{},
			expectedIsFiltered:      false,
		},
		{
			name: "excluded authors filtration",
			upd: domain.RawUpdate{
				ID:          1,
				Description: "123123123123213213132",
				Author:      "author1",
				TgChatIDs:   []int64{1, 2},
			},
			expectedProcessedUpdate: domain.ProcessedUpdate{},
			expectedIsFiltered:      false,
		},
		{
			name: "description contains stop word",
			upd: domain.RawUpdate{
				ID:          1,
				Description: "some text 123213 sadasdx\n\n\n\nstops\n",
				Author:      "1",
				TgChatIDs:   []int64{1, 2},
			},
			expectedProcessedUpdate: domain.ProcessedUpdate{},
			expectedIsFiltered:      false,
		},
		{
			name: "pass filtration",
			upd: domain.RawUpdate{
				ID:          1,
				Description: "123123123123123",
				Author:      "1",
				TgChatIDs:   []int64{1, 2},
			},
			expectedProcessedUpdate: domain.ProcessedUpdate{
				ID:          1,
				Description: "123123123123123",
				TgChatIDs:   []int64{1, 2},
			},
			expectedIsFiltered: true,
		},
		{
			name: "summarization",
			upd: domain.RawUpdate{
				ID:          1,
				Description: "12312312312312313213123123123123123123",
				Author:      "1",
				TgChatIDs:   []int64{1, 2},
			},
			expectedProcessedUpdate: domain.ProcessedUpdate{
				ID:          1,
				Description: "12312312312312313213...",
				TgChatIDs:   []int64{1, 2},
			},
			expectedIsFiltered: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			processedUpd, isFiltered := s.service.filter(&tt.upd)
			s.Equal(tt.expectedProcessedUpdate, processedUpd)
			s.Equal(tt.expectedIsFiltered, isFiltered)
		})
	}
}

func (s *ServiceSuite) TestDeterminePriority() {
	tests := []struct {
		name           string
		description    string
		expectPriority domain.Priority
	}{
		{
			name:           "no low or high keywords",
			description:    "12312312312312313213",
			expectPriority: domain.MediumPriority,
		},
		{
			name:           "high priority",
			description:    "123 123 high",
			expectPriority: domain.HighPriority,
		},
		{
			name:           "low priority",
			description:    "123 123 low",
			expectPriority: domain.LowPriority,
		},
		{
			name:           "low and high keywords",
			description:    "high 123 low",
			expectPriority: domain.HighPriority,
		},
		{
			name:           "keyword is part of another word",
			description:    "123high 123 123",
			expectPriority: domain.MediumPriority,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			priority := s.service.determinePriority(tt.description)
			s.Equal(tt.expectPriority, priority)
		})
	}
}

func (s *ServiceSuite) TestGrouping_SingleUpdate_NoGrouping() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "single update message",
		Author:      "user",
		TgChatIDs:   []int64{12345},
	}

	expectedProcessed := domain.ProcessedUpdate{
		ID:          1,
		Description: "single update message",
		TgChatIDs:   []int64{12345},
		Priority:    domain.MediumPriority,
	}

	done := make(chan struct{})
	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			close(done)
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestGrouping_MultipleUpdatesForSameChat() {
	chatID := int64(12345)

	upd1 := domain.RawUpdate{
		ID:          1,
		Description: "First update message",
		Author:      "user1",
		TgChatIDs:   []int64{chatID},
	}

	upd2 := domain.RawUpdate{
		ID:          2,
		Description: "Second update message",
		Author:      "user2",
		TgChatIDs:   []int64{chatID},
	}

	expectedProcessed := domain.ProcessedUpdate{
		ID:          1,
		Description: "1. First update message\n2. Second update message",
		TgChatIDs:   []int64{chatID},
		Priority:    domain.MediumPriority,
	}

	done := make(chan struct{})
	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			close(done)
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd1)
	s.Require().NoError(err)

	err = s.service.HandleRawUpdate(upd2)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestGrouping_MultipleUpdatesForDifferentChats() {
	chatID1 := int64(12345)
	chatID2 := int64(67890)

	upd1 := domain.RawUpdate{
		ID:          1,
		Description: "Update for chat 1",
		Author:      "user1",
		TgChatIDs:   []int64{chatID1},
	}

	upd2 := domain.RawUpdate{
		ID:          2,
		Description: "Update for chat 2",
		Author:      "user2",
		TgChatIDs:   []int64{chatID2},
	}

	expectedProcessed1 := domain.ProcessedUpdate{
		ID:          1,
		Description: "Update for chat 1",
		TgChatIDs:   []int64{chatID1},
		Priority:    domain.MediumPriority,
	}

	expectedProcessed2 := domain.ProcessedUpdate{
		ID:          2,
		Description: "Update for chat 2",
		TgChatIDs:   []int64{chatID2},
		Priority:    domain.MediumPriority,
	}

	var callCount int
	done := make(chan struct{}, 2)

	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed1).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			callCount++
			done <- struct{}{}
			return nil
		},
	).Times(1)

	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed2).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			callCount++
			done <- struct{}{}
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd1)
	s.Require().NoError(err)

	err = s.service.HandleRawUpdate(upd2)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		return callCount >= 2
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestGrouping_PriorityMaxAmongGroup() {
	chatID := int64(12345)

	updLow := domain.RawUpdate{
		ID:          1,
		Description: "low priority message",
		Author:      "user1",
		TgChatIDs:   []int64{chatID},
	}

	updHigh := domain.RawUpdate{
		ID:          2,
		Description: "high priority message",
		Author:      "user2",
		TgChatIDs:   []int64{chatID},
	}

	expectedProcessed := domain.ProcessedUpdate{
		ID:          1,
		Description: "1. low priority message\n2. high priority message",
		TgChatIDs:   []int64{chatID},
		Priority:    domain.HighPriority,
	}

	done := make(chan struct{})
	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			close(done)
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(updLow)
	s.Require().NoError(err)

	err = s.service.HandleRawUpdate(updHigh)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestGrouping_UpdatesOutsideWindow() {
	chatID := int64(12345)

	upd1 := domain.RawUpdate{
		ID:          1,
		Description: "First update",
		Author:      "user1",
		TgChatIDs:   []int64{chatID},
	}

	upd2 := domain.RawUpdate{
		ID:          2,
		Description: "Second update (outside window)",
		Author:      "user2",
		TgChatIDs:   []int64{chatID},
	}

	expectedProcessed1 := domain.ProcessedUpdate{
		ID:          1,
		Description: "First update",
		TgChatIDs:   []int64{chatID},
		Priority:    domain.MediumPriority,
	}

	expectedProcessed2 := domain.ProcessedUpdate{
		ID:          2,
		Description: "Second update (outside window)",
		TgChatIDs:   []int64{chatID},
		Priority:    domain.MediumPriority,
	}

	var callCount int
	done := make(chan struct{}, 2)

	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed1).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			callCount++
			done <- struct{}{}
			return nil
		},
	).Times(1)

	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed2).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			callCount++
			done <- struct{}{}
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd1)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		return callCount == 1
	}, testGroupingWindow+waitTime, tickTime)

	err = s.service.HandleRawUpdate(upd2)
	s.Require().NoError(err)

	s.Eventually(func() bool {
		return callCount == 2
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestHandleRawUpdate_SendingError() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "123123123123123",
		Author:      "1",
		TgChatIDs:   []int64{1, 2},
	}

	done := make(chan struct{})
	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			close(done)
			return errors.New("some error")
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd)
	s.Require().NoError(err)

	waitTime := 200 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, testGroupingWindow+waitTime, tickTime)
}

func (s *ServiceSuite) TestHandleRawUpdate_MessageFiltered() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "1",
		Author:      "1",
		TgChatIDs:   []int64{1, 2},
	}

	err := s.service.HandleRawUpdate(upd)
	s.Require().NoError(err)
}

func (s *ServiceSuite) TestStop_FlushesRemainingGroups() {
	chatID := int64(12345)

	upd := domain.RawUpdate{
		ID:          1,
		Description: "Message that will be flushed",
		Author:      "user",
		TgChatIDs:   []int64{chatID},
	}

	expectedProcessed := domain.ProcessedUpdate{
		ID:          1,
		Description: "Message that will be flushed",
		TgChatIDs:   []int64{chatID},
		Priority:    domain.MediumPriority,
	}

	done := make(chan struct{})
	s.mockNotifier.EXPECT().SendUpdate(gomock.Any(), expectedProcessed).DoAndReturn(
		func(_ context.Context, _ domain.ProcessedUpdate) error {
			close(done)
			return nil
		},
	).Times(1)

	err := s.service.HandleRawUpdate(upd)
	s.Require().NoError(err)

	s.service.Stop()

	waitTime := 10000 * time.Millisecond
	tickTime := 10 * time.Millisecond
	s.Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, waitTime, tickTime)
}
