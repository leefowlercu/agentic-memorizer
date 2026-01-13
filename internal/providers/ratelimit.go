package providers

import (
	"context"
	"sync"
	"time"
)

// RateLimiter provides token bucket rate limiting for API calls.
type RateLimiter struct {
	mu            sync.Mutex
	tokens        float64
	maxTokens     float64
	refillRate    float64 // tokens per second
	lastRefill    time.Time
	requestTokens float64 // tokens consumed per request
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	maxTokens := float64(config.BurstSize)
	if maxTokens == 0 {
		maxTokens = float64(config.RequestsPerMinute)
	}

	// Convert requests per minute to tokens per second
	refillRate := float64(config.RequestsPerMinute) / 60.0

	return &RateLimiter{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refillRate:    refillRate,
		lastRefill:    time.Now(),
		requestTokens: 1.0,
	}
}

// Wait blocks until a token is available or context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.refill()

		if r.tokens >= r.requestTokens {
			r.tokens -= r.requestTokens
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time
		tokensNeeded := r.requestTokens - r.tokens
		waitTime := time.Duration(tokensNeeded/r.refillRate*1000) * time.Millisecond
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to try again
		}
	}
}

// TryAcquire attempts to acquire a token without blocking.
func (r *RateLimiter) TryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= r.requestTokens {
		r.tokens -= r.requestTokens
		return true
	}

	return false
}

// refill adds tokens based on elapsed time (must be called with lock held).
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
}

// Available returns the current number of available tokens.
func (r *RateLimiter) Available() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()
	return r.tokens
}

// RateLimiterManager manages rate limiters for multiple providers.
type RateLimiterManager struct {
	mu       sync.RWMutex
	limiters map[string]*RateLimiter
}

// NewRateLimiterManager creates a new rate limiter manager.
func NewRateLimiterManager() *RateLimiterManager {
	return &RateLimiterManager{
		limiters: make(map[string]*RateLimiter),
	}
}

// GetOrCreate returns the rate limiter for a provider, creating if needed.
func (m *RateLimiterManager) GetOrCreate(providerName string, config RateLimitConfig) *RateLimiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limiter, exists := m.limiters[providerName]; exists {
		return limiter
	}

	limiter := NewRateLimiter(config)
	m.limiters[providerName] = limiter
	return limiter
}

// Get returns the rate limiter for a provider if it exists.
func (m *RateLimiterManager) Get(providerName string) (*RateLimiter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limiter, exists := m.limiters[providerName]
	return limiter, exists
}
