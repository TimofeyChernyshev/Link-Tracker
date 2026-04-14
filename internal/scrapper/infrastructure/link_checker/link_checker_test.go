package linkchecker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkCheckerSuite struct {
	suite.Suite
	client    *LinkChecker
	server    *httptest.Server
	ctx       context.Context
	batchSize int
}

func (s *LinkCheckerSuite) SetupTest() {
	s.ctx = context.Background()
	s.batchSize = 100
	s.client = NewLinkChecker("test-bot/1.0", s.batchSize, "github", "stack")
}

func (s *LinkCheckerSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestLinkCheckerSuite(t *testing.T) {
	suite.Run(t, new(LinkCheckerSuite))
}
func (s *LinkCheckerSuite) TestCheck_Success_WithChanges() {
	fixedTime := time.Now().UTC()
	lastModified := fixedTime.Format(http.TimeFormat)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodGet, r.Method)
		w.Header().Set("Last-Modified", lastModified)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: fixedTime.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.NotEmpty(events)
	s.Contains(events[0].Description, "updated")
}

func (s *LinkCheckerSuite) TestCheck_Success_NoChanges() {
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	lastModified := fixedTime.Format(http.TimeFormat)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", lastModified)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: fixedTime.Add(1 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Empty(events)
}

func (s *LinkCheckerSuite) TestCheck_NoLastModifiedHeader() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Empty(events)
}

func (s *LinkCheckerSuite) TestCheck_InvalidLastModifiedHeader() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", "invalid-date-format")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "parsing time")
	s.Nil(events)
}

func (s *LinkCheckerSuite) TestCheck_NotFound() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "status code: 404")
	s.Nil(events)
}

func (s *LinkCheckerSuite) TestCheck_ServerError() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "status code: 500")
	s.Nil(events)
}

func (s *LinkCheckerSuite) TestCheck_ContextTimeout() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "context deadline exceeded")
	s.Nil(events)
}

func (s *LinkCheckerSuite) TestCheck_ContextCanceled() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	events, err := s.client.Check(ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "context canceled")
	s.Nil(events)
}
