package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

type ResilientHTTPClient struct {
	client         *http.Client
	retryConfig    RetryConfig
	circuitBreaker *gobreaker.CircuitBreaker
	rateLimiter    *rate.Limiter
}

func NewResilientHTTPClient(retryCfg RetryConfig, cbConfig CircuitBreakerConfig, rateLimit int, timeout time.Duration) *ResilientHTTPClient {
	settings := gobreaker.Settings{
		Name:        cbConfig.Name,
		MaxRequests: cbConfig.MaxRequests,
		Interval:    cbConfig.Interval,
		Timeout:     cbConfig.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= uint32(cbConfig.MinimumNumberOfCalls) && failureRatio >= cbConfig.FailureRateThreshold/100.0
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Info("circuit breaker state changed", "name", name, "from", from, "to", to)
		},
	}

	return &ResilientHTTPClient{
		client:         &http.Client{Timeout: timeout},
		retryConfig:    retryCfg,
		circuitBreaker: gobreaker.NewCircuitBreaker(settings),
		rateLimiter:    rate.NewLimiter(rate.Limit(rateLimit), rateLimit),
	}
}

// Do выполняет HTTP запрос
func (c *ResilientHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Rate Limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Circuit Breaker + Retry
	//nolint:bodyclose // тело ответа закрывается внутри doWithRetry или вызывающей функцией
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.doWithRetry(ctx, req)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker execute: %w", err)
	}

	resp, ok := result.(*http.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return resp, nil
}

func (c *ResilientHTTPClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response

	retryableFunc := func() error {
		reqClone := req.Clone(ctx)

		r, err := c.client.Do(reqClone)
		if err != nil {
			slog.Warn("request failed, will retry", "error", err)
			return fmt.Errorf("cannot do request: %w", err)
		}

		if slices.Contains(c.retryConfig.RetryableHTTP, r.StatusCode) {
			_ = r.Body.Close()
			slog.Warn("retryable status, will retry", "status", r.StatusCode)
			return fmt.Errorf("retryable status code: %d", r.StatusCode)
		}

		resp = r
		return nil
	}

	err := retry.Do(
		retryableFunc,
		retry.Attempts(uint(c.retryConfig.MaxAttempts)),
		retry.Delay(c.retryConfig.InitialDelay),
		retry.MaxDelay(c.retryConfig.MaxDelay),
		retry.DelayType(retry.FixedDelay), // constant backoff
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			slog.Warn("retrying request", "attempt", n, "error", err)
		}),
	)

	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, fmt.Errorf("failed after %d attempts: %w", c.retryConfig.MaxAttempts, err)
	}

	return resp, nil
}
