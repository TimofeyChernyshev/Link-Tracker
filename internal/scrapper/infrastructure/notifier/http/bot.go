package httpnotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

type BotClient struct {
	baseURL    string
	httpClient *resilience.ResilientHTTPClient
}

func NewBotClient(baseURL string, httpClient *resilience.ResilientHTTPClient) *BotClient {
	return &BotClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *BotClient) SendUpdate(ctx context.Context, upd domain.LinkUpdate) error {
	updAPI := LinkUpdate{
		ID:          upd.ID,
		URL:         upd.URL,
		Description: upd.Description,
		TgChatIDs:   upd.ChatIDs,
	}

	body, err := json.Marshal(updAPI)
	if err != nil {
		slog.Error("cannot marshal update", "update", updAPI, "error", err)
		return fmt.Errorf("cannot marshal update: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/updates", bytes.NewReader(body))
	if err != nil {
		slog.Error("cannot create new request", "url", c.baseURL+"/updates", "error", err)
		return fmt.Errorf("cannot create new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		slog.Error("cannot send request", "request", req, "error", err)
		return fmt.Errorf("cannot send request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Error("failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr APIErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&apiErr)
		if err != nil {
			slog.Error("cannot decode api error response", "error", err)
			return fmt.Errorf("cannot decode api error response: %w", err)
		}

		slog.Error("got not OK status code", "statusCode", resp.StatusCode, "description", apiErr.Description)
		return fmt.Errorf("scrapper error (code: %s): %s", apiErr.Code, apiErr.Description)
	}

	return nil
}

func (c *BotClient) Close() error {
	return nil
}
