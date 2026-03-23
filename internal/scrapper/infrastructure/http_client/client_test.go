package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type HTTPClientSuite struct {
	suite.Suite
	client *HTTPClient
	server *httptest.Server
	ctx    context.Context
}

func (s *HTTPClientSuite) SetupTest() {
	s.ctx = context.Background()
	s.client = NewHTTPClient()
}

func (s *HTTPClientSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestHTTPClientSuite(t *testing.T) {
	suite.Run(t, new(HTTPClientSuite))
}
func (s *HTTPClientSuite) TestCheck_Success_WithChanges() {
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

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.True(changed)
	s.Contains(msg, "updated")
}

func (s *HTTPClientSuite) TestCheck_Success_NoChanges() {
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

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.False(changed)
	s.Empty(msg)
}

func (s *HTTPClientSuite) TestCheck_NoLastModifiedHeader() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.False(changed)
	s.Equal("no last-modified header", msg)
}

func (s *HTTPClientSuite) TestCheck_InvalidLastModifiedHeader() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Last-Modified", "invalid-date-format")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "parsing time")
	s.False(changed)
	s.Empty(msg)
}

func (s *HTTPClientSuite) TestCheck_NotFound() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "status code: 404")
	s.False(changed)
	s.Empty(msg)
}

func (s *HTTPClientSuite) TestCheck_ServerError() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.server.Close()

	link := domain.Link{
		URL:       s.server.URL,
		UpdatedAt: time.Now(),
	}

	changed, msg, err := s.client.Check(s.ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "status code: 500")
	s.False(changed)
	s.Empty(msg)
}

func (s *HTTPClientSuite) TestCheck_ContextTimeout() {
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

	changed, msg, err := s.client.Check(ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "context deadline exceeded")
	s.False(changed)
	s.Empty(msg)
}

func (s *HTTPClientSuite) TestCheck_ContextCanceled() {
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

	changed, msg, err := s.client.Check(ctx, link)

	s.Require().Error(err)
	s.Contains(err.Error(), "context canceled")
	s.False(changed)
	s.Empty(msg)
}
