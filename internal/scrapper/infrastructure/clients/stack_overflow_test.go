package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	s.ctx = context.Background()
	s.client = NewStackOverflowClient("test-bot/1.0")
}

func (s *SOClientSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestSOClientSuite(t *testing.T) {
	suite.Run(t, new(SOClientSuite))
}

func (s *SOClientSuite) TestExtractQuestionID() {
	tests := []struct {
		name     string
		url      string
		expected int64
		wantErr  bool
	}{
		{
			name:     "just URL",
			url:      "https://stackoverflow.com/questions/12345",
			expected: 12345,
			wantErr:  false,
		},
		{
			name:     "URL with /",
			url:      "https://stackoverflow.com/questions/12345/",
			expected: 12345,
			wantErr:  false,
		},
		{
			name:     "URL with header",
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
			name:    "invalid ID",
			url:     "https://stackoverflow.com/questions/abc",
			wantErr: true,
		},
		{
			name:    "wrong path",
			url:     "https://stackoverflow.com/12345",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := s.client.extractQuestionID(tt.url)

			if tt.wantErr {
				s.Require().Error(err)
			} else {
				s.NoError(err)
				s.Equal(tt.expected, result)
			}
		})
	}
}

func (s *SOClientSuite) TestExtractSite() {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "eng",
			url:  "https://stackoverflow.com/questions/12345",
			want: "stackoverflow",
		},
		{
			name: "ru",
			url:  "https://ru.stackoverflow.com/questions/12345",
			want: "ru.stackoverflow",
		},
		{
			name: "spanish",
			url:  "https://es.stackoverflow.com/questions/12345",
			want: "es.stackoverflow",
		},
		{
			name: "port",
			url:  "https://pt.stackoverflow.com/questions/12345",
			want: "pt.stackoverflow",
		},
		{
			name: "japanese",
			url:  "https://ja.stackoverflow.com/questions/12345",
			want: "ja.stackoverflow",
		},
		{
			name: "unknown domen",
			url:  "https://example.com/questions/12345",
			want: "stackoverflow",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := s.client.extractSite(tt.url)
			s.Equal(tt.want, result)
		})
	}
}

func (s *SOClientSuite) TestCheck_WithChanges() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("stackoverflow", r.URL.Query().Get("site"))
		s.Equal("!nNPvSNVZMB", r.URL.Query().Get("filter"))
		s.Equal("/2.3/questions/12345", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"items": [{
				"question_id": 12345,
				"title": "Test Question",
				"last_activity_date": ` + strconv.FormatInt(time.Now().Unix(), 10) + `
			}]
		}`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: time.Now().Add(-24 * time.Hour),
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.True(changed)
	s.Contains(desc, "Question updated: Test Question")
}

func (s *SOClientSuite) TestCheck_NoChanges() {
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"items": [{
				"question_id": 12345,
				"title": "Test Question",
				"last_activity_date": ` + strconv.FormatInt(fixedTime.Unix(), 10) + `
			}]
		}`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL:       "https://stackoverflow.com/questions/12345",
		UpdatedAt: fixedTime.Add(1 * time.Hour),
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_NotFound() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items": []}`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/99999",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "not found")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_RateLimit() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/12345",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "rate limit")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_BadRequest() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/12345",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "invalid request")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_ServerError() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/12345",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "returned status 500")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_InvalidJSON() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid": json`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/12345",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_ContextTimeout() {
	s.server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/2.3/questions"

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	link := domain.Link{
		URL: "https://stackoverflow.com/questions/12345",
	}

	changed, desc, err := s.client.Check(ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "context deadline exceeded")
	s.False(changed)
	s.Empty(desc)
}

func (s *SOClientSuite) TestCheck_DifferentSites() {
	sites := []struct {
		url  string
		site string
	}{
		{"https://stackoverflow.com/questions/12345", "stackoverflow"},
		{"https://ru.stackoverflow.com/questions/12345", "ru.stackoverflow"},
		{"https://es.stackoverflow.com/questions/12345", "es.stackoverflow"},
	}

	for _, tt := range sites {
		s.Run(tt.site, func() {
			s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.Equal(tt.site, r.URL.Query().Get("site"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"items": [{
						"question_id": 12345,
						"title": "Test",
						"last_activity_date": ` + strconv.FormatInt(time.Now().Unix(), 10) + `
					}]
				}`))
			}))
			defer s.server.Close()

			s.client.baseURL = s.server.URL + "/2.3/questions"

			link := domain.Link{
				URL:       tt.url,
				UpdatedAt: time.Now().Add(-24 * time.Hour),
			}

			changed, desc, err := s.client.Check(s.ctx, link)

			s.Require().NoError(err)
			s.True(changed)
			s.NotEmpty(desc)
		})
	}
}
