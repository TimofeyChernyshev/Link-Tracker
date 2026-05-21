package application

import (
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

const (
	testFilterMinLenght        = 10
	testsummarizationThreshold = 20
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

func (s *ServiceSuite) TestHandleRawUpdate() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "123123123123123",
		Author:      "1",
		TgChatIDs:   []int64{1, 2},
	}

	processedUpd := domain.ProcessedUpdate{
		ID:          1,
		Description: "123123123123123",
		TgChatIDs:   []int64{1, 2},
		Priority:    domain.MediumPriority,
	}

	s.mockNotifier.EXPECT().SendUpdate(s.ctx, processedUpd).Return(nil)

	err := s.service.HandleRawUpdate(s.ctx, upd)
	s.Require().NoError(err)
}

func (s *ServiceSuite) TestHandleRawUpdate_MessageFiltered() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "1",
		Author:      "1",
		TgChatIDs:   []int64{1, 2},
	}

	err := s.service.HandleRawUpdate(s.ctx, upd)
	s.Require().NoError(err)
}

func (s *ServiceSuite) TestHandleRawUpdate_SendingError() {
	upd := domain.RawUpdate{
		ID:          1,
		Description: "123123123123123",
		Author:      "1",
		TgChatIDs:   []int64{1, 2},
	}

	processedUpd := domain.ProcessedUpdate{
		ID:          1,
		Description: "123123123123123",
		TgChatIDs:   []int64{1, 2},
		Priority:    domain.MediumPriority,
	}

	s.mockNotifier.EXPECT().SendUpdate(s.ctx, processedUpd).Return(errors.New("some error"))

	err := s.service.HandleRawUpdate(s.ctx, upd)
	s.Require().Error(err)
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
