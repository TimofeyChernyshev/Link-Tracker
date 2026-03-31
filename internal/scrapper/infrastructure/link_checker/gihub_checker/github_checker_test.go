package githubchecker

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
				s.Require().Error(err)
				if tt.errMsg != "" {
					s.Require().Contains(err.Error(), tt.errMsg)
				}
			} else {
				s.Require().NoError(err)
				s.Equal(tt.expected, result)
			}
		})
	}
}

func (s *GithubClientSuite) TestCheck_NewPRAndIssue() {
	now := time.Now()
	prTime := now.Add(-1 * time.Hour)
	issueTime := now.Add(-2 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls"):
			prs := []PullRequest{
				{
					ID:        1,
					Number:    123,
					Title:     "pr title",
					User:      User{Login: "123"},
					CreatedAt: prTime,
					Body:      "pr text",
					HTMLURL:   "https://github.com/owner/repo/pull/123",
				},
			}
			json.NewEncoder(w).Encode(prs)
		case strings.Contains(r.URL.Path, "/issues"):
			issues := []Issue{
				{
					ID:        2,
					Number:    456,
					Title:     "issue title",
					User:      User{Login: "234"},
					CreatedAt: issueTime,
					Body:      "issue text",
					HTMLURL:   "https://github.com/owner/repo/issues/456",
				},
			}
			json.NewEncoder(w).Encode(issues)
		default:
			repo := Repository{FullName: "owner/repo", UpdatedAt: now.Add(-3 * time.Hour)}
			json.NewEncoder(w).Encode(repo)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 2)
	s.Contains(events[0].Description, "issue")
	s.Contains(events[1].Description, "pr")
}

func (s *GithubClientSuite) TestCheck_IgnorePullRequestInIssues() {
	now := time.Now()

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls"):
			json.NewEncoder(w).Encode([]PullRequest{})
		case strings.Contains(r.URL.Path, "/issues"):
			issues := []Issue{
				{
					ID:          1,
					Number:      789,
					Title:       "pr in issue",
					User:        User{Login: "user"},
					CreatedAt:   now,
					Body:        "some text",
					PullRequest: &struct{}{},
				},
			}
			json.NewEncoder(w).Encode(issues)
		default:
			repo := Repository{FullName: "owner/repo", UpdatedAt: now}
			json.NewEncoder(w).Encode(repo)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: now.Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Empty(events)
}

func (s *GithubClientSuite) TestCheck_NoNewEvents() {
	now := time.Now()
	lastChecked := now.Add(-1 * time.Hour)

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls"):
			prs := []PullRequest{
				{
					ID:        1,
					Number:    123,
					Title:     "Old PR",
					User:      User{Login: "octocat"},
					CreatedAt: now.Add(-2 * time.Hour),
					Body:      "Old PR",
					HTMLURL:   "https://github.com/owner/repo/pull/123",
				},
			}
			json.NewEncoder(w).Encode(prs)
		default:
			repo := Repository{FullName: "owner/repo", UpdatedAt: now.Add(-2 * time.Hour)}
			json.NewEncoder(w).Encode(repo)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: lastChecked,
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Empty(events)
}

func (s *GithubClientSuite) TestCheck_Truncation() {
	longBody := string(make([]byte, 300))
	for i := range longBody {
		longBody = longBody[:i] + "a"
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/pulls"):
			prs := []PullRequest{
				{
					ID:        1,
					Number:    123,
					Title:     "Long description PR",
					User:      User{Login: "octocat"},
					CreatedAt: time.Now(),
					Body:      longBody,
					HTMLURL:   "https://github.com/owner/repo/pull/123",
				},
			}
			json.NewEncoder(w).Encode(prs)
		default:
			repo := Repository{FullName: "owner/repo", UpdatedAt: time.Now()}
			json.NewEncoder(w).Encode(repo)
		}
	}))
	defer s.server.Close()

	s.client.baseURL = s.server.URL + "/repos"

	link := domain.Link{
		URL:       "https://github.com/owner/repo",
		UpdatedAt: time.Now().Add(-24 * time.Hour),
	}

	events, err := s.client.Check(s.ctx, link)

	s.Require().NoError(err)
	s.Len(events, 1)
	s.Contains(events[0].Description, "...")
}
