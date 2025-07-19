package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// A service that always succeeds.
func successfulService() (string, error) {
	return "Success", nil
}

// A service that always fails.
func failingService() (string, error) {
	return "", errors.New("service failure")
}

func TestGateway_SuccessfulRequest(t *testing.T) {
	t.Parallel()
	config := Config{FailureThreshold: 3, ResetTimeout: 100 * time.Millisecond}
	gateway := NewAPIGateway(config)
	if gateway == nil {
		t.Fatal("NewAPIGateway not implemented yet. Please return a gateway instance.")
	}

	res, err := gateway.Execute(successfulService)
	if err != nil {
		t.Errorf("Expected no error for a successful service call, but got %v", err)
	}
	if res != "Success" {
		t.Errorf("Expected response 'Success', but got '%s'", res)
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	t.Parallel()
	config := Config{FailureThreshold: 3, ResetTimeout: 2 * time.Second}
	gateway := NewAPIGateway(config)
	if gateway == nil {
		t.Fatal("NewAPIGateway not implemented yet. Please return a gateway instance.")
	}

	// Fail 3 times to trip the breaker
	for i := 0; i < config.FailureThreshold; i++ {
		_, err := gateway.Execute(failingService)
		if err == nil {
			t.Fatalf("Expected an error on failure %d, but got none", i+1)
		}
	}

	// The 4th call should fail immediately (circuit is open)
	_, err := gateway.Execute(successfulService) // Use successful service to ensure it's not called
	if err == nil {
		t.Fatal("Expected an error because the circuit should be open, but got none")
	}
	expectedErr := "circuit breaker is open"
	if err.Error() != expectedErr {
		t.Errorf("Expected '%s' error, but got: %v", expectedErr, err)
	}
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	t.Parallel()
	config := Config{FailureThreshold: 2, ResetTimeout: 50 * time.Millisecond}
	gateway := NewAPIGateway(config)
	if gateway == nil {
		t.Fatal("NewAPIGateway not implemented yet. Please return a gateway instance.")
	}

	// Fail 2 times to trip the breaker
	for i := 0; i < config.FailureThreshold; i++ {
		gateway.Execute(failingService)
	}

	// Wait for the reset timeout
	time.Sleep(config.ResetTimeout + 10*time.Millisecond)

	// The next call should be in the half-open state and succeed
	res, err := gateway.Execute(successfulService)
	if err != nil {
		t.Fatalf("Expected the circuit to reset and the call to succeed, but got error: %v", err)
	}
	if res != "Success" {
		t.Errorf("Expected successful response after reset, but got: %s", res)
	}

	// The circuit should be closed now, the next call should also succeed
	_, err = gateway.Execute(successfulService)
	if err != nil {
		t.Errorf("Expected subsequent call to succeed after reset, but got error: %v", err)
	}
}

func TestCircuitBreaker_StaysOpenOnHalfOpenFailure(t *testing.T) {
	t.Parallel()
	config := Config{FailureThreshold: 2, ResetTimeout: 50 * time.Millisecond}
	gateway := NewAPIGateway(config)
	if gateway == nil {
		t.Fatal("NewAPIGateway not implemented yet. Please return a gateway instance.")
	}

	// Trip the breaker
	for i := 0; i < config.FailureThreshold; i++ {
		gateway.Execute(failingService)
	}

	// Wait for the reset timeout to enter half-open state
	time.Sleep(config.ResetTimeout + 10*time.Millisecond)

	// Make a failing call in half-open state, which should re-open the circuit
	_, err := gateway.Execute(failingService)
	if err == nil {
		t.Fatal("Expected the half-open call to fail, but it succeeded")
	}

	// The circuit should be open again immediately
	_, err = gateway.Execute(successfulService)
	if err == nil || err.Error() != "circuit breaker is open" {
		t.Errorf("Expected circuit to re-open after a half-open failure. Error: %v", err)
	}
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	t.Parallel()
	config := Config{FailureThreshold: 10, ResetTimeout: 100 * time.Millisecond}
	gateway := NewAPIGateway(config)
	if gateway == nil {
		t.Fatal("NewAPIGateway not implemented yet. Please return a gateway instance.")
	}

	numRequests := 100
	var wg sync.WaitGroup
	wg.Add(numRequests)

	// Send a mix of successful and failing requests concurrently
	for i := 0; i < numRequests; i++ {
		go func(i int) {
			defer wg.Done()
			service := successfulService
			if i%5 == 0 { // Make some requests fail
				service = failingService
			}
			// We just want to ensure no race conditions or panics.
			gateway.Execute(service)
		}(i)
	}

	wg.Wait()
	// If we reach here without the race detector complaining or a panic, it's a success.
}