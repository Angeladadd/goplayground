package ratelimiter

import (
	"sort"
	"sync"
	"time"
)

// RateLimiter defines the interface for our rate limiter.
type RateLimiter interface {
    Allow(id string) bool
}

type UserWindow struct {
	window []time.Time
	mu sync.Mutex
}

type slidingWindowLimiter struct {
	limit int
	window time.Duration
	userWindows map[string]*UserWindow
	mu sync.RWMutex
}

func New(limit int, window time.Duration) RateLimiter {
    return &slidingWindowLimiter{
		limit: limit,
		window: window,
		userWindows: make(map[string]*UserWindow),
		mu: sync.RWMutex{},
	}
}

func (rl *slidingWindowLimiter) Allow(id string) bool {
	currentTime := time.Now()
	rl.mu.RLock()
	userWindow, exist := rl.userWindows[id]
	rl.mu.RUnlock()
	if !exist {
		rl.mu.Lock()
		rl.userWindows[id] = &UserWindow{
			window: []time.Time{},
			mu: sync.Mutex{},
		}
		userWindow = rl.userWindows[id]
		rl.mu.Unlock()
	}

	userWindow.mu.Lock()
	defer userWindow.mu.Unlock()
	startOfWindow := currentTime.Add(-rl.window)
	first := sort.Search(len(userWindow.window), func(i int) bool {
		return !userWindow.window[i].Before(startOfWindow)
	})
	userWindow.window = userWindow.window[first:]
	if len(userWindow.window) < rl.limit {
		userWindow.window = append(userWindow.window, currentTime)
		return true
	}
	return false
}