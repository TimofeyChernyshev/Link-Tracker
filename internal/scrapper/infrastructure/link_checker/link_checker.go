package linkchecker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	githubchecker "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/link_checker/gihub_checker"
	stackoverflowchecker "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/link_checker/stack_overflow_checker"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

type LinkChecker struct {
	httpClient          *resilience.ResilientHTTPClient
	githubClient        *githubchecker.GithubClient
	stackOverflowClient *stackoverflowchecker.StackOverflowClient
}

func NewLinkChecker(userAgent string, batchSize, previewLen int, githubBaseURL, stackBaseURL string,
	basicClient, githubClient, stackClient *resilience.ResilientHTTPClient) *LinkChecker {
	return &LinkChecker{
		httpClient:          basicClient,
		githubClient:        githubchecker.NewGithubClient(githubBaseURL, userAgent, batchSize, previewLen, githubClient),
		stackOverflowClient: stackoverflowchecker.NewStackOverflowClient(stackBaseURL, userAgent, batchSize, previewLen, stackClient),
	}
}

func (c *LinkChecker) Check(ctx context.Context, link domain.Link) ([]domain.Event, error) {
	switch {
	case strings.Contains(link.URL, "github.com"):
		events, err := c.githubClient.Check(ctx, link)
		if err != nil {
			return nil, fmt.Errorf("checking github client: %w", err)
		}

		return events, nil
	case strings.Contains(link.URL, "stackoverflow.com"):
		events, err := c.stackOverflowClient.Check(ctx, link)
		if err != nil {
			return nil, fmt.Errorf("checking stack overflow client: %w", err)
		}

		return events, nil
	default:
		return c.checkLastModified(ctx, link)
	}
}

func (c *LinkChecker) checkLastModified(ctx context.Context, link domain.Link) ([]domain.Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		return nil, nil
	}

	t, err := http.ParseTime(lastModified)
	if err != nil {
		return nil, fmt.Errorf("parsing time: %w", err)
	}

	if t.After(link.UpdatedAt) {
		return []domain.Event{{
			Description: fmt.Sprintf("%s updated", link.URL),
			OccurredAt:  t,
		}}, nil
	}

	return nil, nil
}
