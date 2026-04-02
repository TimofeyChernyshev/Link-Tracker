package stackoverflowchecker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type SOClientSuite struct {
	suite.Suite
	client *StackOverflowClient
	server *httptest.Server
	ctx    context.Context
}

func (s *SOClientSuite) SetupTest() {
	batchSize := 100

	s.ctx = context.Background()
	s.client = NewStackOverflowClient("test-bot/1.0", batchSize)
}

func (s *SOClientSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestSOClientSuite(t *testing.T) {
	suite.Run(t, new(SOClientSuite))
}

func (s *SOClientSuite) TestCheck_NewAnswer() {
	now := time.Now()
	answerTime := now.Add(-1 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/answers"):
			answersResp := AnswersResponse{
				Items: []Answer{
					{
						AnswerID:     123,
						Owner:        User{DisplayName: "answerer"},
						CreationDate: answerTime.Unix(),
						Body:         "answer text",
					},
				},
			}
			json.NewEncoder(w).Encode(answersResp)
		case strings.Contains(r.URL.Path, "/comments"):
			json.NewEncoder(w).Encode(CommentsResponse{Items: []Comment{}})
		default:
			questionResp := QuestionResponse{
				Items: []Question{
					{QuestionID: 12345, Title: "Test Question"},
				},
			}
			json.NewEncoder(w).Encode(questionResp)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 1)
	s.Contains(events[0].Description, "answerer")
	s.Contains(events[0].Description, "Test Question")
}

func (s *SOClientSuite) TestCheck_NewComment() {
	now := time.Now()
	commentTime := now.Add(-1 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/answers"):
			json.NewEncoder(w).Encode(AnswersResponse{Items: []Answer{}})
		case strings.Contains(r.URL.Path, "/comments"):
			commentsResp := CommentsResponse{
				Items: []Comment{
					{
						CommentID:    456,
						Owner:        User{DisplayName: "commenter"},
						CreationDate: commentTime.Unix(),
						Body:         "comment text",
					},
				},
			}
			json.NewEncoder(w).Encode(commentsResp)
		default:
			questionResp := QuestionResponse{
				Items: []Question{
					{QuestionID: 12345, Title: "Test Question"},
				},
			}
			json.NewEncoder(w).Encode(questionResp)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 1)
	s.Contains(events[0].Description, "commenter")
	s.Contains(events[0].Description, "Test Question")
}

func (s *SOClientSuite) TestCheck_BothAnswerAndComment() {
	now := time.Now()
	answerTime := now.Add(-1 * time.Hour)
	commentTime := now.Add(-2 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/answers"):
			answersResp := AnswersResponse{
				Items: []Answer{
					{
						AnswerID:     123,
						Owner:        User{DisplayName: "answerer"},
						CreationDate: answerTime.Unix(),
						Body:         "Answer text",
					},
				},
			}
			json.NewEncoder(w).Encode(answersResp)
		case strings.Contains(r.URL.Path, "/comments"):
			commentsResp := CommentsResponse{
				Items: []Comment{
					{
						CommentID:    456,
						Owner:        User{DisplayName: "commenter"},
						CreationDate: commentTime.Unix(),
						Body:         "Comment text",
					},
				},
			}
			json.NewEncoder(w).Encode(commentsResp)
		default:
			questionResp := QuestionResponse{
				Items: []Question{
					{QuestionID: 12345, Title: "Test Question"},
				},
			}
			json.NewEncoder(w).Encode(questionResp)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 2)
	s.Contains(events[0].Description, "комментарий")
	s.Contains(events[1].Description, "ответ")
}

func (s *SOClientSuite) TestCheck_NoNewEvents() {
	now := time.Now()
	oldAnswerTime := now.Add(-48 * time.Hour)
	lastChecked := now.Add(-24 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/answers"):
			answersResp := AnswersResponse{
				Items: []Answer{
					{
						AnswerID:     123,
						Owner:        User{DisplayName: "answerer"},
						CreationDate: oldAnswerTime.Unix(),
						Body:         "Old answer",
					},
				},
			}
			json.NewEncoder(w).Encode(answersResp)
		case strings.Contains(r.URL.Path, "/comments"):
			json.NewEncoder(w).Encode(CommentsResponse{Items: []Comment{}})
		default:
			questionResp := QuestionResponse{
				Items: []Question{
					{QuestionID: 12345, Title: "Test Question"},
				},
			}
			json.NewEncoder(w).Encode(questionResp)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: lastChecked,
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Empty(events)
}

func (s *SOClientSuite) TestCheck_Truncation() {
	now := time.Now()
	longBody := string(make([]byte, 300))
	for i := range longBody {
		longBody = longBody[:i] + "a"
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/answers"):
			answersResp := AnswersResponse{
				Items: []Answer{
					{
						AnswerID:     123,
						Owner:        User{DisplayName: "answerer"},
						CreationDate: now.Unix(),
						Body:         longBody,
					},
				},
			}
			json.NewEncoder(w).Encode(answersResp)
		default:
			questionResp := QuestionResponse{
				Items: []Question{
					{QuestionID: 12345, Title: "Test Question"},
				},
			}
			json.NewEncoder(w).Encode(questionResp)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 1)
	s.Contains(events[0].Description, "...")
}

func (s *SOClientSuite) TestExtractQuestionID() {
	tests := []struct {
		name     string
		url      string
		expected int64
		wantErr  bool
	}{
		{
			name:     "basic URL",
			url:      "https://stackoverflow.com/questions/12345",
			expected: 12345,
			wantErr:  false,
		},
		{
			name:     "URL with headers",
			url:      "https://stackoverflow.com/questions/12345/how-to-test",
			expected: 12345,
			wantErr:  false,
		},
		{
			name:     "ru stackoverflow",
			url:      "https://ru.stackoverflow.com/questions/67890",
			expected: 67890,
			wantErr:  false,
		},
		{
			name:    "without ID",
			url:     "https://stackoverflow.com/questions/",
			wantErr: true,
		},
		{
			name:    "unvalid ID",
			url:     "https://stackoverflow.com/questions/abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := s.client.extractQuestionID(tt.url)
			if tt.wantErr {
				s.Error(err)
			} else {
				s.Require().NoError(err)
				s.Equal(tt.expected, result)
			}
		})
	}
}
