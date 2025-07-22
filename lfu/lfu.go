package lfu

import (
	"container/list"
	"sync"
)

// Cache is the interface for a generic LFU cache.
type Cache[K comparable, V any] interface {
	// Put adds a value to the cache.
	// It returns true if an item was evicted.
	Put(key K, value V) (evicted bool)

	// Get retrieves a value from the cache.
	// It returns the value and a boolean indicating whether the key was found.
	Get(key K) (value V, ok bool)

	// Len returns the number of items in the cache.
	Len() int
}

type mynode[K comparable, V any] struct {
	key K
	value V
	freq int
}

type lfuCache[K comparable, V any] struct {
	mu sync.Mutex
	capacity int
	minFreq int
	keyToNode map[K]*list.Element
	freqToList map[int]*list.List
}

func New[K comparable, V any](capacity int) Cache[K, V] {
    return &lfuCache[K, V]{
		mu: sync.Mutex{},
		capacity: capacity,
		minFreq: 0,
		keyToNode: make(map[K]*list.Element),
		freqToList: make(map[int]*list.List),
	}
}

func (c *lfuCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.keyToNode)
}

func (c *lfuCache[K, V]) Put(key K, value V) (evicted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	evicted = false
	node, exists := c.keyToNode[key]
	if !exists {
		if c.capacity == len(c.keyToNode) {
			evicted = true
			// pop out the least frequently used node
			l := c.freqToList[c.minFreq]
			p := l.Front()
			l.Remove(p)
			delete(c.keyToNode, p.Value.(*mynode[K,V]).key)
		}
		// put the new entry in cache
		newnode := &mynode[K, V]{
			key: key,
			value: value,
			freq: 1,
		}
		if _, exists := c.freqToList[1]; !exists {
			c.freqToList[1] = list.New()
		}
		c.freqToList[1].PushBack(newnode)
		c.keyToNode[key] = c.freqToList[1].Back()
		c.minFreq = 1
	} else {
		c.refresh(node)
		entry := node.Value.(*mynode[K, V])
		entry.value = value
	}
	return 
}

func (c *lfuCache[K, V]) refresh(node *list.Element) {
	entry := node.Value.(*mynode[K, V])
	c.freqToList[entry.freq].Remove(node)
	if c.minFreq == entry.freq && c.freqToList[entry.freq].Len() == 0 {
		c.minFreq++
	}
	entry.freq++
	if _, exists := c.freqToList[entry.freq]; !exists {
		c.freqToList[entry.freq] = list.New()
	}
	c.freqToList[entry.freq].PushBack(entry)
	c.keyToNode[entry.key] = c.freqToList[entry.freq].Back()
}

func (c *lfuCache[K, V]) Get(key K) (value V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	node, exists := c.keyToNode[key]
	if !exists {
		ok = false
	} else {
		value, ok = node.Value.(*mynode[K, V]).value, true
		c.refresh(node)
	}
	return 
}