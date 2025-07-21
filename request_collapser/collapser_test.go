package collapser

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Assumes a constructor New() exists
func TestSingleRequest(t *testing.T) {
	c := New()
	var executions int32

	fn := func() (interface{}, error) {
		atomic.AddInt32(&executions, 1)
		return "result", nil
	}

	res, err := c.Do("key1", fn)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res != "result" {
		t.Errorf("Expected result 'result', got %v", res)
	}
	if executions != 1 {
		t.Errorf("Expected fn to be executed once, but was executed %d times", executions)
	}
}

func TestConcurrentRequestsSameKey(t *testing.T) {
	c := New()
	var executions int32

	fn := func() (interface{}, error) {
		atomic.AddInt32(&executions, 1)
		time.Sleep(100 * time.Millisecond)
		return "shared_result", nil
	}

	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			res, err := c.Do("key1", fn)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if res != "shared_result" {
				t.Errorf("Expected result 'shared_result', got %v", res)
			}
		}()
	}

	wg.Wait()

	if executions != 1 {
		t.Fatalf("Expected fn to be executed once, but was executed %d times", executions)
	}
}

func TestErrorPropagation(t *testing.T) {
	c := New()
	var executions int32
	expectedErr := errors.New("something went wrong")

	fn := func() (interface{}, error) {
		atomic.AddInt32(&executions, 1)
		time.Sleep(50 * time.Millisecond)
		return nil, expectedErr
	}

	numGoroutines := 5
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			res, err := c.Do("key1", fn)
			if res != nil {
				t.Errorf("Expected nil result on error, got %v", res)
			}
			if err != expectedErr {
				t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
			}
		}()
	}

	wg.Wait()

	if executions != 1 {
		t.Errorf("Expected fn to be executed once, but was executed %d times", executions)
	}
}

func TestKeyIsCleanedUp(t *testing.T) {
	c := New()
	var executions int32

	fn := func() (interface{}, error) {
		atomic.AddInt32(&executions, 1)
		return "result", nil
	}

	// First call
	_, _ = c.Do("key1", fn)
	// Second call with the same key
	_, _ = c.Do("key1", fn)

	if executions != 2 {
		t.Fatalf("Expected 2 executions, got %d. This implies the key was not cleaned up after the first call completed.", executions)
	}
}