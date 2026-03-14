package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type GithubClientSuite struct {
	suite.Suite
	client *GithubClient
	server *httptest.Server
	ctx    context.Context
}

func (s *GithubClientSuite) SetupTest() {
	s.ctx = context.Background()
	s.client = NewGithubClient("test-bot/1.0")
}

func (s *GithubClientSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
	}
}

func TestGithubClientSuite(t *testing.T) {
	suite.Run(t, new(GithubClientSuite))
}

func (s *GithubClientSuite) TestExtractRepoPath() {
	tests := []struct {
		name     string
		url      string
		expected string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "простой URL",
			url:      "https://github.com/owner/repo",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "URL с слешем в конце",
			url:      "https://github.com/owner/repo/",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "URL с вложенным путем",
			url:      "https://github.com/owner/repo/tree/main/src",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "URL с issues",
			url:      "https://github.com/owner/repo/issues/42",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:     "URL с pulls",
			url:      "https://github.com/owner/repo/pull/123",
			expected: "owner/repo",
			wantErr:  false,
		},
		{
			name:    "недостаточно сегментов",
			url:     "https://github.com/owner",
			wantErr: true,
			errMsg:  "URL must contain owner and repo name",
		},
		{
			name:    "пустой URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "невалидный URL",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result, err := s.client.extractRepoPath(tt.url)

			if tt.wantErr {
				assert.Error(s.T(), err)
				if tt.errMsg != "" {
					assert.Contains(s.T(), err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(s.T(), err)
				assert.Equal(s.T(), tt.expected, result)
			}
		})
	}
}

func (s *GithubClientSuite) TestCheck_WithChanges() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(s.T(), "application/vnd.github.v3+json", r.Header.Get("Accept"))
		assert.Equal(s.T(), "test-bot/1.0", r.Header.Get("User-Agent"))
		assert.Equal(s.T(), "/repos/owner/repo", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"full_name": "owner/repo",
			"updated_at": "` + time.Now().Format(time.RFC3339) + `"
		}`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: time.Now().Add(-24 * time.Hour),
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.NoError(s.T(), err)
	assert.True(s.T(), changed)
	assert.Contains(s.T(), desc, "Repository owner/repo was updated")
}

func (s *GithubClientSuite) TestCheck_NoChanges() {
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"full_name": "owner/repo",
			"updated_at": "` + fixedTime.Format(time.RFC3339) + `"
		}`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: fixedTime.Add(1 * time.Hour),
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.NoError(s.T(), err)
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_NotFound() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL: "https://github.com/owner/nonexistent",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "not found")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_RateLimit() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL: "https://github.com/owner/repo",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "rate limit")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_Unauthorized() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL: "https://github.com/owner/repo",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid or missing token")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_ServerError() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL: "https://github.com/owner/repo",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "returned status 500")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_InvalidJSON() {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"invalid": json`))
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL: "https://github.com/owner/repo",
	}

	changed, desc, err := s.client.Check(s.ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to decode")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}

func (s *GithubClientSuite) TestCheck_ContextTimeout() {
	s.server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	link := domain.Link{
		URL: "https://github.com/owner/repo",
	}

	changed, desc, err := s.client.Check(ctx, link)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "context deadline exceeded")
	assert.False(s.T(), changed)
	assert.Empty(s.T(), desc)
}
