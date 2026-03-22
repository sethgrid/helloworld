package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/sethgrid/helloworld/logger"
)

// rateLimiter implements a simple token bucket rate limiter
type rateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

// newRateLimiter creates a new rate limiter with the specified rate
// rate is requests per second
func newRateLimiter(rate int) *rateLimiter {
	if rate <= 0 {
		rate = 100 // Default to 100 requests per second if invalid
	}
	return &rateLimiter{
		tokens:     rate,
		maxTokens:  rate,
		refillRate: time.Second / time.Duration(rate),
		lastRefill: time.Now(),
	}
}

// allow checks if a request is allowed and consumes a token if available
func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed / rl.refillRate)
	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// rateLimitMiddleware creates a middleware that rate limits requests
func rateLimitMiddleware(requestsPerSecond int) func(http.Handler) http.Handler {
	limiter := newRateLimiter(requestsPerSecond)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow() {
				logger.FromRequest(r).Warn("rate limit exceeded", "ip", r.RemoteAddr)
				w.Header().Set("Retry-After", "1")
				errorHandler(w, r, http.StatusTooManyRequests, "too many requests", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
