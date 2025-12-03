package observer

import (
	"context"
	"sync"
)

// Observer defines the callback contract for receiving published events of type T.
type Observer[T any] interface {
	Notify(context.Context, T) error
}

// ObserverFunc adapts a standalone function into an Observer.
//
//revive:disable-next-line:exported
type ObserverFunc[T any] func(context.Context, T) error

// Notify executes the wrapped function.
func (f ObserverFunc[T]) Notify(ctx context.Context, evt T) error {
	if f == nil {
		return nil
	}
	return f(ctx, evt)
}

// Publisher publishes events to downstream observers.
type Publisher[T any] interface {
	Publish(context.Context, T)
}

// Subject coordinates observer registrations and event fan-out.
type Subject[T any] struct {
	mu        sync.RWMutex
	observers []Observer[T]
	onError   func(error)
}

// NewSubject constructs a Subject with optional initial observers.
func NewSubject[T any](observers ...Observer[T]) *Subject[T] {
	cp := append([]Observer[T](nil), observers...)
	return &Subject[T]{observers: cp}
}

// Publish invokes every observer with the provided event.
func (s *Subject[T]) Publish(ctx context.Context, evt T) {
	if s == nil {
		return
	}

	s.mu.RLock()
	observers := append([]Observer[T](nil), s.observers...)
	errHandler := s.onError
	s.mu.RUnlock()

	for _, obs := range observers {
		if obs == nil {
			continue
		}
		if err := obs.Notify(ctx, evt); err != nil && errHandler != nil {
			errHandler(err)
		}
	}
}

// Attach registers additional observers to the subject.
func (s *Subject[T]) Attach(observers ...Observer[T]) {
	if s == nil || len(observers) == 0 {
		return
	}
	s.mu.Lock()
	s.observers = append(s.observers, observers...)
	s.mu.Unlock()
}

// SetErrorHandler configures a callback for observer failures.
func (s *Subject[T]) SetErrorHandler(fn func(error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onError = fn
	s.mu.Unlock()
}
