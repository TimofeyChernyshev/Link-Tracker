package githubchecker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	GitHubTimeout = 5 * time.Second
	previewLen    = 200
)

type GithubClient struct {
	http      *http.Client
	baseURL   string
	userAgent string
}

func NewGithubClient(userAgent string) *GithubClient {
	return &GithubClient{
		http:      &http.Client{Timeout: GitHubTimeout},
		baseURL:   "https://api.github.com/repos",
		userAgent: userAgent,
	}
}

func (c *GithubClient) Check(ctx context.Context, link domain.Link) ([]domain.Event, error) {
	repoPath, err := c.extractRepoPath(link.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	// repo, err := c.fetchRepository(ctx, fmt.Sprintf("%s/%s", c.baseURL, repoPath))
	// if err != nil {
	// 	return nil, err
	// }

	prs, err := c.fetchPRs(ctx, repoPath, link.UpdatedAt)
	if err != nil {
		slog.Warn("cannot fetch PRs", "error", err)
	}

	issues, err := c.fetchIssues(ctx, repoPath, link.UpdatedAt)
	if err != nil {
		slog.Warn("cannot fetch issues", "error", err)
	}

	var events []domain.Event
	lastUpdated := link.UpdatedAt

	for _, pr := range prs {
		if pr.CreatedAt.After(lastUpdated) {
			events = append(events, domain.Event{
				Description: c.formatPRMessage(&pr),
				OccurredAt:  pr.CreatedAt,
			})
		}
	}

	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		if issue.CreatedAt.After(lastUpdated) {
			events = append(events, domain.Event{
				Description: c.formatIssueMessage(&issue),
				OccurredAt:  issue.CreatedAt,
			})
		}
	}

	// обновление в репозитории, но не pr или issue
	// if repo.UpdatedAt.After(lastUpdated) {
	// 	events = append(events, domain.Event{
	// 		Description: fmt.Sprintf("Repository %s was updated", repo.FullName),
	// 		OccurredAt:  repo.UpdatedAt,
	// 	})
	// }

	sort.Slice(events, func(i, j int) bool {
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})

	return events, nil
}

func (c *GithubClient) formatPRMessage(pr *PullRequest) string {
	preview := truncateString(pr.Body)
	return fmt.Sprintf(
		"Новый Pull Request в репозитории\n\n Название: %s\n Автор: %s\n Время создания: %s\n Описание:\n%s\n\n [Ссылка](%s)",
		pr.Title,
		pr.User.Login,
		pr.CreatedAt.Format("2006-01-02 15:04:05"),
		preview,
		pr.HTMLURL,
	)
}

func (c *GithubClient) formatIssueMessage(issue *Issue) string {
	preview := truncateString(issue.Body)
	return fmt.Sprintf(
		"Новый Issue в репозитории\n\n Название: %s\n Автор: %s\n Время создания: %s\n Описание:\n%s\n\n [Ссылка](%s)",
		issue.Title,
		issue.User.Login,
		issue.CreatedAt.Format("2006-01-02 15:04:05"),
		preview,
		issue.HTMLURL,
	)
}

func truncateString(s string) string {
	if len(s) <= previewLen {
		return s
	}
	return s[:previewLen] + "..."
}

func (c *GithubClient) fetchPRs(ctx context.Context, repoPath string, since time.Time) ([]PullRequest, error) {
	// может быть стоит использовать per_page=20 вместо since
	url := fmt.Sprintf("%s/%s/pulls?state=all&sort=created&direction=desc&since=%s",
		c.baseURL, repoPath, since.Format(time.RFC3339),
	)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing http request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var prs []PullRequest
	err = json.NewDecoder(resp.Body).Decode(&prs)
	return prs, err
}

func (c *GithubClient) fetchIssues(ctx context.Context, repoPath string, since time.Time) ([]Issue, error) {
	url := fmt.Sprintf("%s/%s/issues?state=all&sort=created&direction=desc&since=%s",
		c.baseURL, repoPath, since.Format(time.RFC3339),
	)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var issues []Issue
	err = json.NewDecoder(resp.Body).Decode(&issues)

	return issues, err
}

// func (c *GithubClient) fetchRepository(ctx context.Context, apiURL string) (*Repository, error) {
// 	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create request: %w", err)
// 	}

// 	req.Header.Set("Accept", "application/vnd.github.v3+json")
// 	req.Header.Set("User-Agent", c.userAgent)

// 	resp, err := c.http.Do(req)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to execute request: %w", err)
// 	}
// 	defer func() {
// 		err = resp.Body.Close()
// 		if err != nil {
// 			slog.Warn("cannot close response body", "error", err)
// 		}
// 	}()

// 	if resp.StatusCode != http.StatusOK {
// 		switch resp.StatusCode {
// 		case http.StatusNotFound:
// 			return nil, errors.New("repository not found")
// 		case http.StatusForbidden:
// 			return nil, errors.New("API rate limit exceeded")
// 		case http.StatusUnauthorized:
// 			return nil, errors.New("invalid or missing token")
// 		default:
// 			return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
// 		}
// 	}

// 	var repo Repository
// 	if err = json.NewDecoder(resp.Body).Decode(&repo); err != nil {
// 		return nil, fmt.Errorf("failed to decode response: %w", err)
// 	}

// 	return &repo, nil
// }

func (c *GithubClient) extractRepoPath(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse url: %w", err)
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.Split(path, "/")

	//nolint:mnd // 2 - минимальное число частей URL (owner/repo)
	if len(parts) < 2 {
		return "", errors.New("URL must contain owner and repo name")
	}

	return strings.Join(parts[:2], "/"), nil
}
