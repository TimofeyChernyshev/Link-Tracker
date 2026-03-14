package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type GithubClient struct {
	http      *http.Client
	baseURL   string
	userAgent string
}

func NewGithubClient(userAgent string) *GithubClient {
	return &GithubClient{
		http:      &http.Client{Timeout: 5 * time.Second},
		baseURL:   "https://api.github.com/repos",
		userAgent: userAgent,
	}
}

type repoResp struct {
	FullName  string    `json:"full_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *GithubClient) Check(ctx context.Context, link domain.Link) (bool, string, error) {
	repoPath, err := c.extractRepoPath(link.URL)
	if err != nil {
		return false, "", fmt.Errorf("invalid GitHub URL: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s", c.baseURL, repoPath)

	repo, err := c.fetchRepository(ctx, apiURL)
	if err != nil {
		return false, "", err
	}

	// Единственная логика — сравнение дат
	if repo.UpdatedAt.After(link.UpdatedAt) {
		description := fmt.Sprintf("Repository %s was updated", repo.FullName)
		return true, description, nil
	}

	return false, "", nil
}

func (c *GithubClient) extractRepoPath(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", fmt.Errorf("URL must contain owner and repo name")
	}

	return strings.Join(parts[:2], "/"), nil
}

func (c *GithubClient) fetchRepository(ctx context.Context, apiURL string) (*repoResp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		slog.Error("failed to close response body", "error", err)
	}()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("repository not found")
		case http.StatusForbidden:
			return nil, fmt.Errorf("API rate limit exceeded")
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("invalid or missing token")
		default:
			return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}
	}

	var repo repoResp
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &repo, nil
}
