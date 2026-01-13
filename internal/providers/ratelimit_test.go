package providers

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Wait(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   1000,
		BurstSize:         5,
	}

	rl := NewRateLimiter(config)

	// First requests should succeed immediately
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		start := time.Now()
		err := rl.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait failed: %v", err)
		}
		elapsed := time.Since(start)
		if elapsed > 50*time.Millisecond {
			t.Errorf("burst request %d took too long: %v", i, elapsed)
		}
	}
}

func TestRateLimiter_WaitContextCanceled(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 1, // Very slow
		TokensPerMinute:   1000,
		BurstSize:         1,
	}

	rl := NewRateLimiter(config)

	// Exhaust burst
	ctx := context.Background()
	_ = rl.Wait(ctx)

	// Cancel context before next request
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(cancelCtx)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestRateLimiter_Available(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   1000,
		BurstSize:         5,
	}

	rl := NewRateLimiter(config)

	// Should have burst available
	if rl.Available() <= 0 {
		t.Error("expected limiter to have tokens available initially")
	}
}

func TestRateLimiter_TryAcquire(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   1000,
		BurstSize:         3,
	}

	rl := NewRateLimiter(config)

	// Should succeed for burst
	for i := 0; i < 3; i++ {
		if !rl.TryAcquire() {
			t.Errorf("TryAcquire should succeed for request %d within burst", i)
		}
	}

	// Should fail when burst exhausted
	if rl.TryAcquire() {
		t.Error("TryAcquire should fail when burst exhausted")
	}
}

func TestRateLimiterManager_GetOrCreate(t *testing.T) {
	manager := NewRateLimiterManager()

	config := RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   1000,
		BurstSize:         5,
	}

	// First call creates
	rl1 := manager.GetOrCreate("test", config)
	if rl1 == nil {
		t.Fatal("expected rate limiter to be created")
	}

	// Second call returns same instance
	rl2 := manager.GetOrCreate("test", config)
	if rl1 != rl2 {
		t.Error("expected same rate limiter instance")
	}
}

func TestRateLimiterManager_Get(t *testing.T) {
	manager := NewRateLimiterManager()

	config := RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   1000,
		BurstSize:         5,
	}

	// Should not exist initially
	_, exists := manager.Get("test")
	if exists {
		t.Error("expected limiter to not exist")
	}

	// Create it
	manager.GetOrCreate("test", config)

	// Should exist now
	rl, exists := manager.Get("test")
	if !exists {
		t.Error("expected limiter to exist")
	}
	if rl == nil {
		t.Error("expected non-nil rate limiter")
	}
}
