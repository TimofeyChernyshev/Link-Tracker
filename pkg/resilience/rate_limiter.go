package resilience

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiterMiddleware - middleware для HTTP сервера
type RateLimiterMiddleware struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rps      int
	burst    int
}

func NewRateLimiterMiddleware(rps, burst int) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		limiters: make(map[string]*rate.Limiter),
		rps:      rps,
		burst:    burst,
	}
}

func (rl *RateLimiterMiddleware) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters[ip] = limiter
	}
	return limiter
}

// Middleware возвращает HTTP 429 при превышении лимита
func (rl *RateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"code":"TOO_MANY_REQUESTS","description":"Rate limit exceeded. Please try again later."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
