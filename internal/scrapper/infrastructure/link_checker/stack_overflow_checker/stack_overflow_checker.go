package stackoverflowchecker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	StackOverflowTimeout = 5 * time.Second
	previewLen           = 200
)

var (
	errQuestionNotFound = errors.New("question not found")
)

type StackOverflowClient struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	batchSize  int
}

func NewStackOverflowClient(userAgent string, batchSize int) *StackOverflowClient {
	return &StackOverflowClient{
		httpClient: &http.Client{
			Timeout: StackOverflowTimeout,
		},
		baseURL:   "https://api.stackexchange.com/2.3/questions",
		userAgent: userAgent,
		batchSize: batchSize,
	}
}

func (c *StackOverflowClient) Check(ctx context.Context, link domain.Link) ([]domain.Event, error) {
	questionID, err := c.extractQuestionID(link.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid StackOverflow URL: %w", err)
	}

	site := c.extractSite(link.URL)

	question, err := c.fetchQuestion(ctx, questionID, site)
	if err != nil {
		return nil, err
	}

	answers, err := c.fetchNewAnswers(ctx, questionID, site, link.UpdatedAt)
	if err != nil {
		slog.Warn("cannot fetch answers", "error", err)
		answers = []Answer{}
	}

	comments, err := c.fetchNewComments(ctx, questionID, site, link.UpdatedAt)
	if err != nil {
		slog.Warn("cannot fetch comments", "error", err)
		comments = []Comment{}
	}

	var events []domain.Event

	for _, answer := range answers {
		answer.Title = question.Title
		events = append(events, domain.Event{
			Description: c.formatAnswerMessage(&answer),
			OccurredAt:  time.Unix(answer.CreationDate, 0),
		})
	}

	// Добавляем события для новых комментариев
	for _, comment := range comments {
		events = append(events, domain.Event{
			Description: c.formatCommentMessage(&comment, question.Title),
			OccurredAt:  time.Unix(comment.CreationDate, 0),
		})
	}

	// Сортируем по времени
	sort.Slice(events, func(i, j int) bool {
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})

	return events, nil
}

func (c *StackOverflowClient) formatAnswerMessage(answer *Answer) string {
	preview := truncateString(answer.Body)
	return fmt.Sprintf(
		"Новый ответ на вопрос\n\n Вопрос: %s\n Автор: %s\n Время создания: %s\n Текст ответа:\n%s\n\n"+
			"[Ссылка](https://stackoverflow.com/q/%d#answer-%d)",
		answer.Title,
		answer.Owner.DisplayName,
		time.Unix(answer.CreationDate, 0).Format("2006-01-02 15:04:05"),
		preview,
		answer.QuestionID,
		answer.AnswerID,
	)
}

func (c *StackOverflowClient) formatCommentMessage(comment *Comment, questionTitle string) string {
	preview := truncateString(comment.Body)
	return fmt.Sprintf(
		"Новый комментарий к вопросу\n\n Вопрос: %s\n Автор: %s\n Время создания: %s\n Текст комментария:\n%s",
		questionTitle,
		comment.Owner.DisplayName,
		time.Unix(comment.CreationDate, 0).Format("2006-01-02 15:04:05"),
		preview,
	)
}

func truncateString(s string) string {
	if len(s) <= previewLen {
		return s
	}
	return s[:previewLen] + "..."
}

func (c *StackOverflowClient) fetchQuestion(ctx context.Context, questionID int64, site string) (*Question, error) {
	apiURL := fmt.Sprintf("%s/%d", c.baseURL, questionID)

	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := reqURL.Query()
	q.Add("site", site)
	q.Add("filter", "!nNPvSNVZMB") // фильтр для получения заголовка
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var qResp QuestionResponse
	if err := json.NewDecoder(resp.Body).Decode(&qResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(qResp.Items) == 0 {
		return nil, errors.New("question not found")
	}

	return &qResp.Items[0], nil
}

func (c *StackOverflowClient) fetchNewAnswers(ctx context.Context, questionID int64, site string, since time.Time) ([]Answer, error) {
	apiURL := fmt.Sprintf("%s/%d/answers", c.baseURL, questionID)

	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := reqURL.Query()
	q.Add("site", site)
	q.Add("order", "desc")
	q.Add("sort", "creation")
	q.Add("filter", "withbody")
	q.Add("pagesize", strconv.Itoa(c.batchSize))
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var answersResp AnswersResponse
	if err := json.NewDecoder(resp.Body).Decode(&answersResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var newAnswers []Answer
	for _, answer := range answersResp.Items {
		if time.Unix(answer.CreationDate, 0).After(since) {
			answer.QuestionID = questionID
			newAnswers = append(newAnswers, answer)
		}
	}

	return newAnswers, nil
}

func (c *StackOverflowClient) fetchNewComments(ctx context.Context, questionID int64, site string, since time.Time) ([]Comment, error) {
	apiURL := fmt.Sprintf("%s/%d/comments", c.baseURL, questionID)

	reqURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := reqURL.Query()
	q.Add("site", site)
	q.Add("order", "desc")
	q.Add("sort", "creation")
	q.Add("filter", "withbody")
	q.Add("pagesize", strconv.Itoa(c.batchSize))
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			slog.Warn("cannot close response", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var commentsResp CommentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&commentsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var newComments []Comment
	for _, comment := range commentsResp.Items {
		if time.Unix(comment.CreationDate, 0).After(since) {
			newComments = append(newComments, comment)
		}
	}

	return newComments, nil
}

func (c *StackOverflowClient) extractQuestionID(rawURL string) (int64, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, fmt.Errorf("cannot parse url: %w", err)
	}

	path := strings.Trim(parsed.Path, "/")
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "questions" && i+1 < len(parts) {
			id, err := strconv.ParseInt(parts[i+1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid question ID: %s", parts[i+1])
			}
			return id, nil
		}
	}

	return 0, errors.New("question ID not found in URL")
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
