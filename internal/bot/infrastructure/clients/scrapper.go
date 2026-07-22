package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

const (
	HeaderChatID = "Tg-Chat-Id"
)

type MetricsCollector interface {
	RecordCommandDuration(ctx context.Context, scope, scopeType string, durationMs float64)
}

type ScrapperClient struct {
	baseURL    string
	httpClient *resilience.ResilientHTTPClient
	metrics    MetricsCollector
}

func NewScrapperClient(baseURL string, httpClient *resilience.ResilientHTTPClient, metrics MetricsCollector) *ScrapperClient {
	return &ScrapperClient{
		baseURL:    baseURL,
		httpClient: httpClient,
		metrics:    metrics,
	}
}

func (c *ScrapperClient) AddLink(ctx context.Context, chatID int64, url string, tags []string) error {
	start := time.Now()
	reqBody := AddLinkRequest{
		Link: url,
		Tags: tags,
	}

	respBody := &LinkResponse{}

	err := c.doJSON(ctx, http.MethodPost, chatID, "/links", reqBody, respBody)
	if c.metrics != nil {
		defer func() {
			c.metrics.RecordCommandDuration(ctx, "scrapper_sync_api", "add_link", float64(time.Since(start).Milliseconds()))
		}()
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *ScrapperClient) RemoveLink(ctx context.Context, chatID int64, url string) error {
	start := time.Now()
	reqBody := RemoveLinkRequest{
		Link: url,
	}

	respBody := &LinkResponse{}

	err := c.doJSON(ctx, http.MethodDelete, chatID, "/links", reqBody, respBody)
	if c.metrics != nil {
		defer func() {
			c.metrics.RecordCommandDuration(ctx, "scrapper_sync_api", "remove_link", float64(time.Since(start).Milliseconds()))
		}()
	}
	if err != nil {
		return err
	}

	return nil
}

func (c *ScrapperClient) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	start := time.Now()
	respBody := &ListLinksResponse{}

	url := fmt.Sprintf("/links?limit=%d&offset=%d", limit, offset)

	err := c.doJSON(ctx, http.MethodGet, chatID, url, nil, respBody)
	if c.metrics != nil {
		defer func() {
			c.metrics.RecordCommandDuration(ctx, "scrapper_sync_api", "get_links", float64(time.Since(start).Milliseconds()))
		}()
	}
	if err != nil {
		return nil, err
	}

	links := []domain.Link{}
	for _, l := range respBody.Links {
		links = append(links, domain.Link{URL: l.URL, Tags: l.Tags})
	}

	return links, nil
}

func (c *ScrapperClient) RegisterChat(ctx context.Context, chatID int64) error {
	start := time.Now()
	err := c.doJSON(ctx, http.MethodPost, 0, "/tg-chat/"+strconv.FormatInt(chatID, 10), nil, nil)
	if c.metrics != nil {
		defer func() {
			c.metrics.RecordCommandDuration(ctx, "scrapper_sync_api", "register_chat", float64(time.Since(start).Milliseconds()))
		}()
	}

	return err
}

func (c *ScrapperClient) DeleteChat(ctx context.Context, chatID int64) error {
	start := time.Now()
	err := c.doJSON(ctx, http.MethodDelete, 0, "/tg-chat/"+strconv.FormatInt(chatID, 10), nil, nil)
	if c.metrics != nil {
		defer func() {
			c.metrics.RecordCommandDuration(ctx, "scrapper_sync_api", "delete_chat", float64(time.Since(start).Milliseconds()))
		}()
	}

	return err
}

func (c *ScrapperClient) doJSON(ctx context.Context, method string, chatID int64, url string, reqBody any, respBody any) error {
	var body io.Reader

	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			slog.Error("cannot marshal", "body", reqBody, "error", err)
			return fmt.Errorf("cannot marshal request body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+url, body)
	if err != nil {
		slog.Error("cannot create request", "url", c.baseURL+url, "Method", method, "error", err)
		return fmt.Errorf("cannot create request: %w", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if chatID != 0 {
		req.Header.Set(HeaderChatID, strconv.FormatInt(chatID, 10))
	}

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		slog.Error("cannot send request or get response", "error", err)
		return fmt.Errorf("cannot send request or get response: %w", err)
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
			return fmt.Errorf("cannot decode body: %w", err)
		}

		slog.Error("got not OK status code", "statusCode", resp.StatusCode)
		return fmt.Errorf("scrapper error (code: %s): %s", apiErr.Code, apiErr.Description)
	}

	if respBody != nil {
		err = json.NewDecoder(resp.Body).Decode(respBody)
		if err != nil {
			return fmt.Errorf("cannot decode body: %w", err)
		}
	}

	return nil
}
