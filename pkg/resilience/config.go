package resilience

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// RetryConfig конфигурация для retry
type RetryConfig struct {
	MaxAttempts   int           `env:"RETRY_MAX_ATTEMPTS" envDefault:"3"`
	InitialDelay  time.Duration `env:"RETRY_INITIAL_DELAY" envDefault:"100ms"`
	MaxDelay      time.Duration `env:"RETRY_MAX_DELAY" envDefault:"1s"`
	BackoffFactor float64       `env:"RETRY_BACKOFF_FACTOR" envDefault:"1.0"`
	RetryableHTTP []int         `env:"RETRYABLE_HTTP_STATUSES" envDefault:"500,502,503,504"`
}

func LoadRetryConfig(name string) (*CircuitBreakerConfig, error) {
	var cfg *CircuitBreakerConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse retry config: %w", err)
	}
	cfg.Name = name
	return cfg, nil
}

// CircuitBreakerConfig конфигурация для circuit breaker
type CircuitBreakerConfig struct {
	Name                                  string
	MaxRequests                           uint32        `env:"MAX_REQUESTS" envDefault:"5"`
	Interval                              time.Duration `env:"INTERVAL" envDefault:"0s"`
	Timeout                               time.Duration `env:"TIMEOUT" envDefault:"1s"`
	FailureThreshold                      uint32        `env:"FAILURE_THRESHOLD" envDefault:"5"`
	SuccessThreshold                      uint32        `env:"SUCCESS_THRESHOLD" envDefault:"2"`
	FailureRateThreshold                  float64       `env:"FAILURE_RATE_THRESHOLD" envDefault:"50"`
	MinimumNumberOfCalls                  int           `env:"MINIMUM_CALLS" envDefault:"10"`
	SlidingWindowSize                     int           `env:"WINDOW_SIZE" envDefault:"10"`
	PermittedNumberOfCallsInHalfOpenState int           `env:"PERMITTED_CALLS" envDefault:"5"`
	WaitDurationInOpenState               time.Duration `env:"WAIT_DURATION" envDefault:"1s"`
}

// LoadCircuitBreakerConfig - загрузка конфига для Circuit Breaker
//
// name - имя CircuitBreaker
func LoadCircuitBreakerConfig(name string) (*CircuitBreakerConfig, error) {
	var cfg *CircuitBreakerConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse circuit breaker config: %w", err)
	}
	cfg.Name = name
	return cfg, nil
}

// RateLimiterConfig конфигурация для rate limiter
type RateLimiterConfig struct {
	RPS   int `env:"RPS" envDefault:"50"`
	Burst int `env:"BURST" envDefault:"100"`
}

func LoadRateLimiterConfig() (*RateLimiterConfig, error) {
	var cfg RateLimiterConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse rate limiter config: %w", err)
	}
	return &cfg, nil
}
