package lfu

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLFUCache_SimplePutGet(t *testing.T) {
	cache := New[string, int](2) // Capacity of 2

	cache.Put("a", 1)
	if cache.Len() != 1 {
		t.Errorf("expected len 1, got %d", cache.Len())
	}

	cache.Put("b", 2)
	if cache.Len() != 2 {
		t.Errorf("expected len 2, got %d", cache.Len())
	}

	val, ok := cache.Get("a")
	if !ok || val != 1 {
		t.Errorf("expected to get a=1, got val=%v ok=%v", val, ok)
	}
}

func TestLFUCache_Eviction(t *testing.T) {
	cache := New[string, int](2)

	cache.Put("a", 1) // freq(a)=1
	cache.Put("b", 2) // freq(b)=1

	// Access 'a' to increase its frequency
	cache.Get("a") // freq(a)=2, freq(b)=1

	// Add 'c', which should evict 'b' (the LFU item)
	evicted := cache.Put("c", 3)
	if !evicted {
		t.Errorf("expected an item to be evicted")
	}
	if cache.Len() != 2 {
		t.Errorf("expected len 2 after eviction, got %d", cache.Len())
	}

	// 'b' should be gone
	_, ok := cache.Get("b")
	if ok {
		t.Error("expected key 'b' to be evicted")
	}

	// 'a' and 'c' should be present
	_, ok = cache.Get("a")
	if !ok {
		t.Error("expected key 'a' to be present")
	}
	_, ok = cache.Get("c")
	if !ok {
		t.Error("expected key 'c' to be present")
	}
}

func TestLFUCache_EvictionTieBreaking(t *testing.T) {
	cache := New[string, int](2)

	cache.Put("a", 1) // freq=1, recently used
	time.Sleep(10 * time.Millisecond) // ensure 'b' is more recent
	cache.Put("b", 2) // freq=1, most recently used

	// Frequencies are tied (both 1). Adding 'c' should evict the
	// least recently used of the two, which is 'a'.
	cache.Put("c", 3)

	_, ok := cache.Get("a")
	if ok {
		t.Error("expected 'a' to be evicted due to being LRU among the LFU items")
	}
	_, ok = cache.Get("b")
	if !ok {
		t.Error("expected 'b' to remain")
	}
}


func TestLFUCache_UpdateValue(t *testing.T) {
	cache := New[string, int](1)
	cache.Put("a", 1)
	val, ok := cache.Get("a")
	if !ok || val != 1 {
		t.Errorf("expected a=1, got %v", val)
	}

	// Update the value for 'a'
	cache.Put("a", 100)
	if cache.Len() != 1 {
		t.Errorf("expected len 1 after update, got %d", cache.Len())
	}

	val, ok = cache.Get("a")
	if !ok || val != 100 {
		t.Errorf("expected a=100 after update, got %v", val)
	}
}


func TestLFUCache_Concurrency(t *testing.T) {
	capacity := 100
	cache := New[int, int](capacity)

	var wg sync.WaitGroup
	numGoroutines := 200
	itemsPerGoroutine := 50

	// Concurrent Puts and Gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				key := (g*itemsPerGoroutine + j) % (capacity * 2) // create some key collision and eviction
				cache.Put(key, key)
				cache.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Final check on cache length
	if cache.Len() > capacity {
		t.Errorf("cache length %d exceeds capacity %d", cache.Len(), capacity)
	}

	// Run a final sanity check read
	var readWg sync.WaitGroup
	for i:= 0; i < 10; i++ {
		readWg.Add(1)
		go func() {
			defer readWg.Done()
			for j:=0; j < 100; j++ {
				cache.Get(j)
			}
		}()
	}
	readWg.Wait()
	
	fmt.Printf("Final cache length after concurrent test: %d\n", cache.Len())
}

// You will need to implement a New function and a struct that satisfies the Cache interface.
// For example:
//
// type lfuCache[K comparable, V any] struct { /* ... */ }
//
// func New[K comparable, V any](capacity int) Cache[K, V] {
//     return &lfuCache[K, V]{ /* ... */ }
// }