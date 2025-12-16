package misc

import "sync"

// Resetter is an interface for types that can reset their state.
type Resetter interface {
	Reset()
}

// Pool is a generic object pool for types that implement the Resetter interface.
type Pool[T Resetter] struct {
	p sync.Pool
}

// NewPool creates a new Pool for the specified type T.
func NewPool[T Resetter](newFn func() T) *Pool[T] {
	pl := &Pool[T]{}
	pl.p.New = func() any {
		if newFn != nil {
			return newFn()
		}
		var zero T
		return zero
	}
	return pl
}

// Get retrieves an object from the pool.
func (pl *Pool[T]) Get() T {
	obj := pl.p.Get()
	if value, ok := obj.(T); ok {
		return value
	}
	var zero T
	return zero
}

// Put returns an object to the pool after resetting it.
func (pl *Pool[T]) Put(v T) {
	v.Reset()
	pl.p.Put(v)
}
