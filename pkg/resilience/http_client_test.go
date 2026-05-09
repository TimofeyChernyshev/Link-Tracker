package resilience

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResilientHTTPClient_RetryOn5xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		attempt := atomic.LoadInt32(&attempts)

		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500, 502, 503, 504},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit := 100
	testTimeout := 5 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := resilientClient.Do(context.Background(), req)

	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// должно быть 3 ретрая
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

// TestResilientHTTPClient_NoRetry - нет ретрая, если кода ошибки нет в списке ошибок, которые можно ретраить
func TestResilientHTTPClient_NoRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500, 502, 503, 504},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit := 100
	testTimeout := 5 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := resilientClient.Do(context.Background(), req)

	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestResilientHTTPClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit := 100
	testTimeout := 1 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := resilientClient.Do(context.Background(), req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500},
	}
	cbConfig := CircuitBreakerConfig{
		Name:                                  "test-cb",
		MaxRequests:                           5,
		Timeout:                               2 * time.Second,
		MinimumNumberOfCalls:                  5,
		FailureRateThreshold:                  50,
		PermittedNumberOfCallsInHalfOpenState: 2,
		WaitDurationInOpenState:               1 * time.Second,
	}
	rateLimit := 100
	testTimeout := 1 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		_, err := resilientClient.Do(context.Background(), req)
		require.Error(t, err)
	}

	// После 5 ошибок Circuit Breaker должен быть OPEN
	// Следующий запрос должен завершиться мгновенно без обращения к серверу
	beforeAttempts := atomic.LoadInt32(&attempts)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	_, err := resilientClient.Do(context.Background(), req)
	require.Error(t, err)

	afterAttempts := atomic.LoadInt32(&attempts)
	assert.Equal(t, beforeAttempts, afterAttempts)
}

func TestRateLimiting_ExceedsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   1,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit := 2
	testTimeout := 1 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)

	for i := 0; i < 3; i++ {
		go func() {
			_, err := resilientClient.Do(context.Background(), req)
			_ = err
		}()
	}

	assert.NotPanics(t, func() {
		for i := 0; i < 5; i++ {
			resilientClient.rateLimiter.Allow()
		}
	})
}

func TestResilientHTTPClient_ContextCancellationDuringRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   10,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     2 * time.Second,
	}
	rateLimit := 100
	testTimeout := 5 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	_, err := resilientClient.Do(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")

	assert.LessOrEqual(t, atomic.LoadInt32(&attempts), int32(3))
}

func TestResilientHTTPClient_RetryOnNetworkError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			hijacker, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hijacker.Hijack()
				conn.Close()
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	retryCfg := RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 1.0,
		RetryableHTTP: []int{500, 502, 503, 504},
	}
	cbConfig := CircuitBreakerConfig{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     1 * time.Second,
	}
	rateLimit := 100
	testTimeout := 5 * time.Second

	resilientClient := NewResilientHTTPClient(retryCfg, cbConfig, rateLimit, testTimeout)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := resilientClient.Do(context.Background(), req)

	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}
