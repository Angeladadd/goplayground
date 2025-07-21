package collapser

import "sync"

// Collapser is an interface for a concurrent request collapser.
// It allows multiple goroutines to request the same resource, but ensures
// that only one underlying request is made.
type Collapser interface {
	// Do executes the given function `fn` for a given key.
	//
	// If multiple calls to Do are made with the same key while an initial
	// call is in flight, the function `fn` will only be executed once.
	// All callers will receive the same result and error from the single
	// execution.
	Do(key string, fn func() (interface{}, error)) (interface{}, error)
}

type call struct {
	result interface{}
	err error
	done chan struct{}
}

type myCollapser struct {
	inflightCall map[string]*call
	mu sync.Mutex
}

func New() Collapser {
	return &myCollapser{
		inflightCall: make(map[string]*call),
		mu: sync.Mutex{},
	}
}

func (c *myCollapser) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	c.mu.Lock()
	existingCall, exists := c.inflightCall[key]
	if !exists {
		c.inflightCall[key] = &call{
			done: make(chan struct{}),
		}
		existingCall = c.inflightCall[key]
	}
	c.mu.Unlock()
	if !exists {
		// perform the function and broadcast the result
		result, err := fn()
		existingCall.result = result
		existingCall.err = err
		c.mu.Lock()
		delete(c.inflightCall, key)
		c.mu.Unlock()
		close(existingCall.done)
		return result, err
	} else {
		// wait for the inflight call 
		<-existingCall.done
		return existingCall.result, existingCall.err
	}
}