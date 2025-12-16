package misc

import (
	"sync"
	"testing"
)

type mockResetter struct {
	resetCalled bool
}

func (m *mockResetter) Reset() {
	m.resetCalled = true
}

func TestNewPool(t *testing.T) {
	pool := NewPool(func() *mockResetter {
		return &mockResetter{}
	})

	if pool == nil {
		t.Fatal("expected pool to be created, got nil")
	}
}

func TestPoolGet(t *testing.T) {
	pool := NewPool(func() *mockResetter {
		return &mockResetter{}
	})

	item := pool.Get()
	if item == nil {
		t.Fatal("expected item to be non-nil, got nil")
	}
}

func TestPoolPut(t *testing.T) {
	pool := NewPool(func() *mockResetter {
		return &mockResetter{}
	})

	// Put an item into the pool - Reset should be called on it
	item := &mockResetter{resetCalled: false}
	pool.Put(item)

	// After Put, the item should have Reset called
	if !item.resetCalled {
		t.Fatal("expected Reset to be called on item when Put, but it wasn't")
	}
}

func TestPoolConcurrency(t *testing.T) {
	pool := NewPool(func() *mockResetter {
		return &mockResetter{}
	})

	var wg sync.WaitGroup
	const numGoroutines = 100

	wg.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			item := pool.Get()
			pool.Put(item)
		}()
	}

	wg.Wait()
}
