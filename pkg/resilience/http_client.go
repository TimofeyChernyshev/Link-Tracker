package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

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
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.doWithRetry(ctx, req)
	})

	if err != nil {
		return nil, err
	}

	resp, ok := result.(*http.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return resp, nil
}

func (c *ResilientHTTPClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var lastErr error

	backoff := c.retryConfig.InitialDelay

	for attempt := 1; attempt <= c.retryConfig.MaxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// constant backoff (Factor = 1.0)
				if c.retryConfig.BackoffFactor > 1.0 {
					backoff = time.Duration(float64(backoff) * c.retryConfig.BackoffFactor)
					if backoff > c.retryConfig.MaxDelay {
						backoff = c.retryConfig.MaxDelay
					}
				}
			}
		}

		reqClone := req.Clone(ctx)

		resp, lastErr = c.client.Do(reqClone)
		if lastErr != nil {
			slog.Warn("request failed", "attempt", attempt, "error", lastErr)
			continue
		}

		if slices.Contains(c.retryConfig.RetryableHTTP, resp.StatusCode) {
			closeErr := resp.Body.Close()
			if closeErr != nil {
				slog.Warn("failed to close response body", "error", closeErr)
			}
			slog.Warn("retryable status", "attempt", attempt, "status", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	if resp != nil {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed after %d attempts: %w", c.retryConfig.MaxAttempts, lastErr)
	}

	return nil, fmt.Errorf("failed after %d attempts: all responses returned retryable status codes", c.retryConfig.MaxAttempts)
}
