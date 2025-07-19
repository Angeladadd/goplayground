package distributedid_test

import (
	"sync"
	"testing"
	"time"
	. "playground/distributed_id"
)

// TestNewGenerator checks that the constructor validates the worker ID properly.
func TestNewGenerator(t *testing.T) {
	// Test case 1: A valid worker ID should not return an error.
	validWorkerID := int64(555)
	_, err := NewSnowflakeGenerator(validWorkerID)
	if err != nil {
		t.Errorf("NewSnowflakeGenerator() with valid worker ID %d returned an error: %v", validWorkerID, err)
	}

	// Test case 2: A worker ID that is too large should return an error.
	invalidWorkerID := int64(2000) // max is 1023
	_, err = NewSnowflakeGenerator(invalidWorkerID)
	if err == nil {
		t.Errorf("NewSnowflakeGenerator() with invalid worker ID %d should have returned an error, but did not", invalidWorkerID)
	}

	// Test case 3: A negative worker ID should return an error.
	negativeWorkerID := int64(-1)
	_, err = NewSnowflakeGenerator(negativeWorkerID)
	if err == nil {
		t.Errorf("NewSnowflakeGenerator() with negative worker ID %d should have returned an error, but did not", negativeWorkerID)
	}
}

// TestNextID_Uniqueness ensures that all generated IDs are unique in a single-threaded context.
func TestNextID_Uniqueness(t *testing.T) {
	generator, _ := NewSnowflakeGenerator(1)
	
	// Generate a large number of IDs.
	iterations := 10000
	idMap := make(map[int64]bool, iterations)

	for i := 0; i < iterations; i++ {
		id, err := generator.NextID()
		if err != nil {
			t.Fatalf("NextID() returned an error: %v", err)
		}
		if idMap[id] {
			t.Fatalf("Generated a duplicate ID: %d", id)
		}
		idMap[id] = true
	}
}

// TestNextID_Sortable checks that IDs are generally time-sortable.
func TestNextID_Sortable(t *testing.T) {
	generator, _ := NewSnowflakeGenerator(1)

	id1, err := generator.NextID()
	if err != nil {
		t.Fatalf("NextID() returned an error: %v", err)
	}

	// Wait for a different millisecond.
	time.Sleep(5 * time.Millisecond)

	id2, err := generator.NextID()
	if err != nil {
		t.Fatalf("NextID() returned an error: %v", err)
	}

	if id2 <= id1 {
		t.Errorf("Second ID (%d) is not greater than first ID (%d)", id2, id1)
	}
}

// TestNextID_Concurrency checks for race conditions and duplicate IDs under concurrent load.
func TestNextID_Concurrency(t *testing.T) {
	generator, _ := NewSnowflakeGenerator(1)
	
	numGoroutines := 50
	idsPerGoroutine := 200
	totalIDs := numGoroutines * idsPerGoroutine
	
	var wg sync.WaitGroup
	idChan := make(chan int64, totalIDs)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := generator.NextID()
				if err != nil {
					// t.Errorf() is not safe to call from multiple goroutines.
					// We'll send a sentinel value and check later.
					idChan <- -1 
					return
				}
				idChan <- id
			}
		}()
	}

	wg.Wait()
	close(idChan)

	idMap := make(map[int64]bool, totalIDs)
	for id := range idChan {
		if id == -1 {
			t.Fatal("A goroutine reported an error during ID generation")
		}
		if idMap[id] {
			t.Fatalf("Found a duplicate ID in concurrency test: %d", id)
		}
		idMap[id] = true
	}
}
