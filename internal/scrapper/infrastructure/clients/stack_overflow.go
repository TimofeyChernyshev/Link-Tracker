package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const StackOverflowTimeout = 5 * time.Second

type StackOverflowClient struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// QuestionResponse содержит только необходимые поля
type QuestionResponse struct {
	Items []struct {
		QuestionID       int64  `json:"question_id"`
		Title            string `json:"title"`
		LastActivityDate int64  `json:"last_activity_date"` // только это поле важно
	} `json:"items"`
}

func NewStackOverflowClient(userAgent string) *StackOverflowClient {
	return &StackOverflowClient{
		httpClient: &http.Client{
			Timeout: StackOverflowTimeout,
		},
		baseURL:   "https://api.stackexchange.com/2.3/questions",
		userAgent: userAgent,
	}
}

func (c *StackOverflowClient) Check(ctx context.Context, link domain.Link) (bool, string, error) {
	questionID, err := c.extractQuestionID(link.URL)
	if err != nil {
		return false, "", fmt.Errorf("invalid StackOverflow URL: %w", err)
	}

	site := c.extractSite(link.URL)
	apiURL := fmt.Sprintf("%s/%d", c.baseURL, questionID)

	question, err := c.fetchQuestion(ctx, apiURL, site)
	if err != nil {
		return false, "", err
	}

	if len(question.Items) == 0 {
		return false, "", fmt.Errorf("question not found")
	}

	lastActivity := time.Unix(question.Items[0].LastActivityDate, 0)

	if lastActivity.After(link.UpdatedAt) {
		description := fmt.Sprintf("Question updated: %s", question.Items[0].Title)
		return true, description, nil
	}

	return false, "", nil
}

func (c *StackOverflowClient) extractQuestionID(rawURL string) (int64, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}

	// Разбиваем путь на части
	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	// Ищем сегмент "questions" и следующий за ним
	for i, part := range parts {
		if part == "questions" && i+1 < len(parts) {
			var id int64
			id, err = strconv.ParseInt(parts[i+1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid question ID: %s", parts[i+1])
			}
			return id, nil
		}
	}

	return 0, fmt.Errorf("question ID not found in URL")
}

func (c *StackOverflowClient) extractSite(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "stackoverflow"
	}

	host := parsed.Host
	switch {
	case strings.Contains(host, "ru.stackoverflow"):
		return "ru.stackoverflow"
	case strings.Contains(host, "es.stackoverflow"):
		return "es.stackoverflow"
	case strings.Contains(host, "pt.stackoverflow"):
		return "pt.stackoverflow"
	case strings.Contains(host, "ja.stackoverflow"):
		return "ja.stackoverflow"
	default:
		return "stackoverflow"
	}
}

func (c *StackOverflowClient) fetchQuestion(ctx context.Context, apiURL string, site string) (*QuestionResponse, error) {
	// Добавляем query параметры
	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	q := reqURL.Query()
	q.Add("site", site)
	q.Add("filter", "!nNPvSNVZMB")

	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		slog.Error("failed to close response body", "error", err)
	}()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, fmt.Errorf("question not found")
		case http.StatusTooManyRequests:
			return nil, fmt.Errorf("API rate limit exceeded")
		case http.StatusBadRequest:
			return nil, fmt.Errorf("invalid request parameters")
		default:
			return nil, fmt.Errorf("StackExchange API returned status %d", resp.StatusCode)
		}
	}

	var qResp QuestionResponse
	if err = json.NewDecoder(resp.Body).Decode(&qResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(qResp.Items) == 0 {
		return nil, fmt.Errorf("question not found")
	}

	return &qResp, nil
}
