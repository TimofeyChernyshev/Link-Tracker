package httpclient

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const timeout = 10 * time.Second

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{Timeout: timeout},
	}
}

func (c *HTTPClient) Check(ctx context.Context, link domain.Link) (bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link.URL, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		return false, "no last-modified header", nil
	}

	t, err := http.ParseTime(lastModified)
	if err != nil {
		return false, "", fmt.Errorf("parsing time: %w", err)
	}

	if t.After(link.UpdatedAt) {
		updateMessage := fmt.Sprintf("%s updated", link.URL)
		return true, updateMessage, nil
	}

	return false, "", nil
}
