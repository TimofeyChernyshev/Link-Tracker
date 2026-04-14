package httpnotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const BotTimeout = 5 * time.Second

type BotClient struct {
	baseURL string
	http    *http.Client
}

func NewBotClient(baseURL string) *BotClient {
	return &BotClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: BotTimeout},
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

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Error("cannot send request", "request", req, "error", err)
		return fmt.Errorf("cannot send request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		slog.Error("failed to close response body", "error", err)
	}()

	if resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr APIErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)

		slog.Error("got not OK status code", "statusCode", resp.StatusCode, "description", apiErr.Description)
		return fmt.Errorf("scrapper error (code: %s): %s", apiErr.Code, apiErr.Description)
	}

	return nil
}
