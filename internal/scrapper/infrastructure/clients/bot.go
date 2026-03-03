package clients

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

type BotClient struct {
	baseURL string
	http    *http.Client
}

func NewBotClient(baseURL string) *BotClient {
	return &BotClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *BotClient) SendUpdate(ctx context.Context, upd domain.LinkUpdate) error {
	updAPI := LinkUpdate{
		Id:          upd.Id,
		Url:         upd.Url,
		Description: upd.Description,
		TgChatIds:   upd.TgChatIds,
	}

	body, err := json.Marshal(updAPI)
	if err != nil {
		slog.Error("cannot marshal update", "update", updAPI, "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/updates", bytes.NewReader(body))
	if err != nil {
		slog.Error("cannot create new request", "url", c.baseURL+"/updates", "error", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Error("cannot send request", "request", req, "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var apiErr ApiErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)

		slog.Error("got not OK status code", "statusCode", resp.StatusCode, "description", apiErr.Description)
		return fmt.Errorf("scrapper error (code: %s): %s", apiErr.Code, apiErr.Description)
	}

	return nil
}
