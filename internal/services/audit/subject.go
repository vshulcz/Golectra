package audit

import (
	"context"
	"sync"
)

// Observer receives audit events.
type Observer interface {
	Notify(context.Context, Event) error
}

// ObserverFunc adapts a plain function to the Observer interface.
type ObserverFunc func(context.Context, Event) error

// Notify calls the wrapped function unless it is nil.
func (f ObserverFunc) Notify(ctx context.Context, evt Event) error {
	if f == nil {
		return nil
	}
	return f(ctx, evt)
}

// Publisher broadcasts audit events.
type Publisher interface {
	Publish(context.Context, Event)
}

// Subject fans out events to registered observers.
type Subject struct {
	mu        sync.RWMutex
	observers []Observer
	onError   func(error)
}

// NewSubject creates a subject optionally pre-populated with observers.
func NewSubject(observers ...Observer) *Subject {
	return &Subject{observers: append([]Observer(nil), observers...)}
}

// Publish delivers the event to every observer, shielding the caller from observer errors.
func (s *Subject) Publish(ctx context.Context, evt Event) {
	if s == nil {
		return
	}

	s.mu.RLock()
	observers := append([]Observer(nil), s.observers...)
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

// Attach registers additional observers at runtime.
func (s *Subject) Attach(observers ...Observer) {
	if s == nil {
		return
	}
	if len(observers) == 0 {
		return
	}
	s.mu.Lock()
	s.observers = append(s.observers, observers...)
	s.mu.Unlock()
}

// SetErrorHandler configures a callback for observer failures.
func (s *Subject) SetErrorHandler(fn func(error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onError = fn
	s.mu.Unlock()
}
