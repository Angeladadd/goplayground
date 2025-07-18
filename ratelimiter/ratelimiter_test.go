package ratelimiter_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// This assumes your code is in a module, e.g., github.com/your-name/ratelimiter
	// and the test is in the same directory.
	// For a local setup, you might use "ratelimiter"
	. "playground/ratelimiter"
)

func TestRateLimiter_AllowsFirstRequest(t *testing.T) {
	t.Parallel()
	rl := New(10, time.Minute)

	if !rl.Allow("user-1") {
		t.Error("First request for a user should always be allowed")
	}
}


func TestRateLimiter_RejectsAfterLimit(t *testing.T) {
	t.Parallel()
	limit := 5
	rl := New(limit, time.Minute)
	userID := "user-2"

	// First 5 requests should be allowed
	for i := 0; i < limit; i++ {
		if !rl.Allow(userID) {
			t.Errorf("Request %d should have been allowed, but was rejected", i+1)
		}
	}

	// The 6th request should be rejected
	if rl.Allow(userID) {
		t.Error("Request just over the limit should have been rejected, but was allowed")
	}
}


func TestRateLimiter_WindowResets(t *testing.T) {
	t.Parallel()
	window := 100 * time.Millisecond
	rl := New(2, window)
	userID := "user-3"

	// These two should be allowed
	rl.Allow(userID)
	rl.Allow(userID)

	// This one should be rejected
	if rl.Allow(userID) {
		t.Fatal("Request over limit within window was allowed")
	}

	// Wait for the window to expire
	time.Sleep(window + (5 * time.Millisecond))

	// After the window has passed, a new request should be allowed
	if !rl.Allow(userID) {
		t.Error("Request after window reset should have been allowed")
	}
}

func TestRateLimiter_MultipleUsersAreIndependent(t *testing.T) {
	t.Parallel()
	rl := New(1, time.Minute)
	userA := "user-a"
	userB := "user-b"

	// Allow one request for user A
	if !rl.Allow(userA) {
		t.Fatal("First request for user-a failed")
	}

	// This next request for user A should be rejected
	if rl.Allow(userA) {
		t.Fatal("Second request for user-a should have been rejected")
	}

	// A request for user B should be allowed, as they are a different user
	if !rl.Allow(userB) {
		t.Error("First request for user-b was rejected due to user-a's limit")
	}
}

func TestRateLimiter_Concurrency(t *testing.T) {
	t.Parallel()
	limit := 100
	rl := New(limit, time.Minute)
	userID := "concurrent-user"

	var wg sync.WaitGroup
	var allowedCount int32

	// Number of concurrent requests to simulate
	numRequests := 500
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			if rl.Allow(userID) {
				// Atomically increment the counter for thread safety
				atomic.AddInt32(&allowedCount, 1)
			}
		}()
	}

	wg.Wait()

	if allowedCount != int32(limit) {
		t.Errorf("Expected exactly %d allowed requests, but got %d", limit, allowedCount)
	}

	// Final check: the next request must fail
	if rl.Allow(userID) {
		t.Error("A request after the concurrent burst filled the limit should be rejected")
	}
}


func TestRateLimiter_ZeroLimit(t *testing.T) {
	t.Parallel()
	rl := New(0, time.Minute)
	if rl.Allow("user-zero") {
		t.Error("A rate limiter with a zero limit should not allow any requests")
	}
}

func TestRateLimiter_PreciseWindow(t *testing.T) {
	t.Parallel()
	limit := 1
	window := 200 * time.Millisecond
	rl := New(limit, window)
	userID := "user-precise"

	// 1. First request is allowed
	if !rl.Allow(userID) {
		t.Fatal("First request was not allowed")
	}

	// 2. Wait for half the window
	time.Sleep(window / 2)

	// 3. This request should be denied as it's within the first window
	if rl.Allow(userID) {
		t.Fatal("Second request inside window was allowed")
	}

	// 4. Wait for the remainder of the window plus a small buffer
	time.Sleep((window / 2) + (10 * time.Millisecond))

	// 5. This request should be allowed as the first request's window is over
	if !rl.Allow(userID) {
		t.Error("Request after a precise window expiry was not allowed")
	}
}