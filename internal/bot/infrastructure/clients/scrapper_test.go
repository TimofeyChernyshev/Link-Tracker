package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

var (
	retryCfg = resilience.RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500, 502, 503, 504},
	}
	cbConfig = resilience.CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit = 100
	timeout   = 5 * time.Second
)

func TestAddLink_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCommonHeaders(t, r, http.MethodPost, 123123)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req AddLinkRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "https://github.com/test", req.Link)
		assert.Equal(t, []string{"tag1", "tag2"}, req.Tags)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LinkResponse{
			ID:   1,
			URL:  "https://github.com/test",
			Tags: []string{"tag1", "tag2"},
		})
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.AddLink(t.Context(), 123123, "https://github.com/test", []string{"tag1", "tag2"})
	require.NoError(t, err)
}

func TestAddLink_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errorResponse(w, http.StatusBadRequest, "INVALID_LINK", "Invalid link format")
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.AddLink(t.Context(), 12345, "invalid", []string{"tag1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_LINK")
	assert.Contains(t, err.Error(), "Invalid link format")
}

func TestRemoveLink_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCommonHeaders(t, r, http.MethodDelete, 12345)

		var req RemoveLinkRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "https://github.com/test", req.Link)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LinkResponse{
			ID:   1,
			URL:  "https://github.com/test",
			Tags: []string{},
		})
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.RemoveLink(t.Context(), 12345, "https://github.com/test")
	require.NoError(t, err)
}

func TestGetLinks_Success(t *testing.T) {
	expectedLinks := []domain.Link{
		{URL: "https://github.com/1", Tags: []string{"tag1"}},
		{URL: "https://github.com/2", Tags: []string{"tag2", "tag3"}},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCommonHeaders(t, r, http.MethodGet, 12345)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListLinksResponse{
			Links: []LinkResponse{
				{ID: 1, URL: "https://github.com/1", Tags: []string{"tag1"}},
				{ID: 2, URL: "https://github.com/2", Tags: []string{"tag2", "tag3"}},
			},
			Size: 2,
		})
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	links, err := client.GetLinks(t.Context(), 12345, 50, 0)

	require.NoError(t, err)
	assert.Equal(t, expectedLinks, links)
}

func TestGetLinks_Empty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCommonHeaders(t, r, http.MethodGet, 12345)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ListLinksResponse{
			Links: []LinkResponse{},
			Size:  0,
		})
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	links, err := client.GetLinks(t.Context(), 12345, 10, 20)

	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestRegisterChat_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/tg-chat/12345", r.URL.Path)
		assert.Empty(t, r.Header.Get(HeaderChatID))

		w.WriteHeader(http.StatusOK)
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.RegisterChat(t.Context(), 12345)
	require.NoError(t, err)
}

func TestRegisterChat_AlreadyExists(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errorResponse(w, http.StatusConflict, "CHAT_ALREADY_EXISTS", "Chat already exists")
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.RegisterChat(t.Context(), 12345)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CHAT_ALREADY_EXISTS")
}

func TestDeleteChat_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/tg-chat/12345", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.DeleteChat(t.Context(), 12345)
	require.NoError(t, err)
}

func TestDeleteChat_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errorResponse(w, http.StatusNotFound, "CHAT_NOT_FOUND", "Chat not found")
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.DeleteChat(t.Context(), 12345)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CHAT_NOT_FOUND")
}

func TestTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	scrapperTimeout := 50 * time.Millisecond
	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, scrapperTimeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	err := client.RegisterChat(t.Context(), 12345)
	require.Error(t, err)
}

func TestContextCancel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := client.RegisterChat(ctx, 12345)
	require.Error(t, err)
}

func TestInvalidJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	_, err := client.GetLinks(t.Context(), 12345, 10, 20)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestConcurrent(t *testing.T) {
	var requestCount int32
	var errorCount int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	})

	resilientClient := resilience.NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, timeout)
	server := httptest.NewServer(handler)
	client := NewScrapperClient(server.URL, resilientClient, nil)

	wgCount := 10

	var wg sync.WaitGroup
	wg.Add(wgCount)

	for i := range wgCount {
		go func(id int) {
			defer wg.Done()
			err := client.RegisterChat(t.Context(), int64(id))
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(wgCount), atomic.LoadInt32(&requestCount))
	assert.Equal(t, int32(0), atomic.LoadInt32(&errorCount))
}

// assertCommonHeaders проверяет базовые заголовки запроса
func assertCommonHeaders(t *testing.T, r *http.Request, expectedMethod string, expectedChatID int64) {
	assert.Equal(t, expectedMethod, r.Method)
	assert.Equal(t, "/links", r.URL.Path)

	if expectedChatID != 0 {
		assert.Equal(t, strconv.FormatInt(expectedChatID, 10), r.Header.Get(HeaderChatID))
	}
}

// errorResponse создает JSON ответ с ошибкой
func errorResponse(w http.ResponseWriter, statusCode int, code, description string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIErrorResponse{
		Code:        code,
		Description: description,
	})
}
