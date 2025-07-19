package circuitbreaker

import (
	"fmt"
	"sync"
	"time"
)

// DownstreamService is a function representing a call to a remote service.
// It might succeed and return a response, or fail and return an error.
type DownstreamService func() (string, error)

// APIGateway is the interface for our API gateway with a circuit breaker.
type APIGateway interface {
	// Execute attempts to call the downstream service.
	// It will apply circuit breaker logic to protect the service from overload.
	Execute(service DownstreamService) (string, error)
}

// Config holds the configuration for the circuit breaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures needed to open the circuit.
	FailureThreshold int
	// ResetTimeout is the duration the circuit stays open before transitioning to half-open.
	ResetTimeout time.Duration
}

// TODO: Define a struct that implements the APIGateway interface.
// It should hold the state of the circuit breaker, such as the current state,
// failure counts, and any necessary timestamps. Remember to consider concurrency!
//
type serviceState int
const (
	Close serviceState = iota
	Open
)
type circuitBreakerGateway struct {
	mu sync.Mutex
	state serviceState
	failureCnt int
	lastFailureTime time.Time
	config Config
}

// NewAPIGateway is the constructor that our tests will use.
// You need to implement this function to return an instance of your gateway.
func NewAPIGateway(config Config) APIGateway {
	return &circuitBreakerGateway{
		mu: sync.Mutex{},
		state: Close,
		failureCnt: 0,
		lastFailureTime: time.Time{},
		config: config,
	}
}

func (g *circuitBreakerGateway) Execute(service DownstreamService) (ret string, err error) {
	g.mu.Lock()

	state := g.state
	switch state {
	case Close:
		// unlock mutex to allow concurrent requests
		g.mu.Unlock()
		ret, err = service()
		// update status
		g.mu.Lock()
		if err != nil {
			g.failureCnt += 1
			g.lastFailureTime = time.Now()
			if g.failureCnt >= g.config.FailureThreshold {
				g.state = Open
			}
		} else {
			g.failureCnt = 0
		}
		g.mu.Unlock()
	case Open:
		// check the expiration of open state
		if g.lastFailureTime.Add(g.config.ResetTimeout).Before(time.Now()) {
			// half open logic
			g.mu.Unlock()
			ret, err = service()
			g.mu.Lock()
			if err != nil {
				g.lastFailureTime = time.Now()
			} else {
				g.state = Close
				g.failureCnt = 0
			}
			g.mu.Unlock()
		} else {
			g.mu.Unlock()
			ret, err =  "", fmt.Errorf("circuit breaker is open")
		}
	}
	return
}

