// shutdowner_test.go
package shutdown

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestGracefulShutdowner_HappyPath(t *testing.T) {
	s := New() // We'll need to implement this New() constructor
	var counter int32

	// Add a few simple hooks that increment a counter.
	s.Add(func(ctx context.Context) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&counter, 1)
	})
	s.Add(func(ctx context.Context) {
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&counter, 1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() returned an unexpected error: %v", err)
	}

	if atomic.LoadInt32(&counter) != 2 {
		t.Errorf("Expected counter to be 2, but got %d", counter)
	}
}

func TestGracefulShutdowner_Timeout(t *testing.T) {
	s := New()
	var counter int32

	// This hook will finish
	s.Add(func(ctx context.Context) {
		select {
		case <-time.After(10 * time.Millisecond):
			// Sleep finished normally
			atomic.AddInt32(&counter, 1)
		case <-ctx.Done():
			// Context was cancelled, return immediately
			return
		}
	})
	// This hook will not finish in time
	s.Add(func(ctx context.Context) {
		select {
		case <-time.After(100 * time.Millisecond):
			// Sleep finished normally
			atomic.AddInt32(&counter, 1)
		case <-ctx.Done():
			// Context was cancelled, return immediately
			return
		}
	})

	// The context timeout is shorter than the second hook's sleep time.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected context.DeadlineExceeded error, but got %v", err)
	}

	// Give the goroutines a moment to potentially finish to avoid race condition on counter
	time.Sleep(100 * time.Millisecond)

	// We only expect the first hook to have completed.
	if atomic.LoadInt32(&counter) != 1 {
		t.Errorf("Expected counter to be 1 after timeout, but got %d", counter)
	}
}

func TestGracefulShutdowner_NoHooks(t *testing.T) {
	s := New()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown() with no hooks should return no error, but got: %v", err)
	}
}

func TestGracefulShutdowner_HookRespectsContextCancellation(t *testing.T) {
	s := New()
	hookCompleted := make(chan struct{})

	s.Add(func(ctx context.Context) {
		select {
		case <-time.After(200 * time.Millisecond):
			t.Error("Hook did not return after context was cancelled")
		case <-ctx.Done():
			// This is the expected path.
		}
		close(hookCompleted)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected context.DeadlineExceeded error, but got %v", err)
	}

	// Wait for the hook to finish to check if it respected the cancellation
	<-hookCompleted
}