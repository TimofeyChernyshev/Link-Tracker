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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg"
)

type GithubClient struct {
	http       *http.Client
	baseURL    string
	userAgent  string
	batchSize  int
	previewLen int
}

func NewGithubClient(baseURL, userAgent string, batchSize, previewLen int, timeout time.Duration) *GithubClient {
	return &GithubClient{
		http:       &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		userAgent:  userAgent,
		batchSize:  batchSize,
		previewLen: previewLen,
	}
}

func (c *GithubClient) Check(ctx context.Context, link domain.Link) ([]domain.Event, error) {
	repoPath, err := c.extractRepoPath(link.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid GitHub URL: %w", err)
	}

	var prs []PullRequest
	err = c.fetchItems(ctx, repoPath, "pulls", link.UpdatedAt, &prs)
	if err != nil {
		slog.Warn("cannot fetch PRs", "error", err)
	}

	var issues []Issue
	err = c.fetchItems(ctx, repoPath, "issues", link.UpdatedAt, &issues)
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

	sort.Slice(events, func(i, j int) bool {
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})

	return events, nil
}

func (c *GithubClient) formatPRMessage(pr *PullRequest) string {
	preview := pkg.TruncateString(pr.Body, c.previewLen)
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
	preview := pkg.TruncateString(issue.Body, c.previewLen)
	return fmt.Sprintf(
		"Новый Issue в репозитории\n\n Название: %s\n Автор: %s\n Время создания: %s\n Описание:\n%s\n\n [Ссылка](%s)",
		issue.Title,
		issue.User.Login,
		issue.CreatedAt.Format("2006-01-02 15:04:05"),
		preview,
		issue.HTMLURL,
	)
}

func (c *GithubClient) fetchItems(ctx context.Context, repoPath, itemCategory string, since time.Time, result interface{}) error {
	url := fmt.Sprintf("%s/%s/%s?state=all&sort=created&direction=desc&since=%s&per_page=%d",
		c.baseURL, repoPath, itemCategory, since.Format(time.RFC3339), c.batchSize,
	)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cannot do http request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("cannot decoode: %w", err)
	}

	return nil
}

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
