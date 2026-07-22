package resilience

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterMiddleware_AllowRequestsWithinLimit(t *testing.T) {
	rps := 10
	burst := 5
	rateLimiter := NewRateLimiterMiddleware(rps, burst)

	var requestCount int32
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	})

	handler := rateLimiter.Middleware(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	ip := "192.168.1.100"

	for range burst {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	assert.Equal(t, int32(burst), atomic.LoadInt32(&requestCount))
}

func TestRateLimiterMiddleware_ExceedLimit(t *testing.T) {
	rps := 10
	burst := 3
	rateLimiter := NewRateLimiterMiddleware(rps, burst)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rateLimiter.Middleware(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	ip := "192.168.1.101"

	for range burst {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		req.Header.Set("X-Forwarded-For", ip)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("X-Forwarded-For", ip)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "1", resp.Header.Get("Retry-After"))

	var responseBody map[string]string
	err = json.NewDecoder(resp.Body).Decode(&responseBody)
	require.NoError(t, err)
	assert.Equal(t, "TOO_MANY_REQUESTS", responseBody["code"])
	assert.Equal(t, "Rate limit exceeded. Please try again later.", responseBody["description"])
}

func TestRateLimiterMiddleware_DifferentIPs(t *testing.T) {
	rps := 10
	burst := 2
	rateLimiter := NewRateLimiterMiddleware(rps, burst)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rateLimiter.Middleware(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	ip1 := "192.168.1.200"
	ip2 := "192.168.1.201"

	for range burst {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		req.Header.Set("X-Forwarded-For", ip1)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	req1, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req1.Header.Set("X-Forwarded-For", ip1)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp1.StatusCode)

	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("X-Forwarded-For", ip2)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestRateLimiterMiddleware_RecoveryAfterTime(t *testing.T) {
	rps := 1
	burst := 1
	rateLimiter := NewRateLimiterMiddleware(rps, burst)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rateLimiter.Middleware(testHandler)
	server := httptest.NewServer(handler)
	defer server.Close()

	ip := "192.168.1.202"

	req1, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req1.Header.Set("X-Forwarded-For", ip)
	resp1, err := http.DefaultClient.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("X-Forwarded-For", ip)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp2.StatusCode)

	time.Sleep(1100 * time.Millisecond)

	req3, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req3.Header.Set("X-Forwarded-For", ip)
	resp3, err := http.DefaultClient.Do(req3)
	require.NoError(t, err)
	defer resp3.Body.Close()
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
}
