package shutdown

import (
	"context"
	"fmt"
	"sync"
)

// Shutdowner manages and executes shutdown hooks for a service.
type Shutdowner interface {
	// Add adds a function to be executed on shutdown.
	// These functions are expected to take a context to handle cancellation.
	Add(hook func(ctx context.Context))

	// Shutdown initiates the graceful shutdown process.
	// It executes all registered hooks concurrently.
	// It returns an error if the provided context is cancelled before
	// all hooks have completed.
	Shutdown(ctx context.Context) error
}

type myShutdowner struct {
	mu sync.Mutex
	hooks []func(context.Context)
}

func New() Shutdowner {
	return &myShutdowner{
		mu: sync.Mutex{},
		hooks: []func(context.Context){},
	}
}

func (s *myShutdowner) Add(hook func(ctx context.Context)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hooks = append(s.hooks, hook)
}


func (s *myShutdowner) Shutdown(ctx context.Context) error {
	waitGroupChan := make(chan struct{})
	s.mu.Lock()
	copiedHooks := make([]func(context.Context), len(s.hooks))
	copy(copiedHooks, s.hooks)
	s.mu.Unlock()
	go func (hooks []func(context.Context)) {
		wg := sync.WaitGroup{}
		wg.Add(len(hooks))
		for _, hook := range hooks {
			go func(f func(context.Context)) {
				defer wg.Done()
				defer func() {
					if err := recover(); err != nil {
						fmt.Printf("failed to shutdown: %v\n", err)
					}
				}()
				f(ctx)
			}(hook)
		}
		wg.Wait()
		close(waitGroupChan)
	}(copiedHooks)
	select {
	case <- waitGroupChan:
		return nil
	case <- ctx.Done():
		return ctx.Err()
	}
}