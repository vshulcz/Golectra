package audit

import (
	"context"
	"sync"
)

type Observer interface {
	Notify(context.Context, Event) error
}

type ObserverFunc func(context.Context, Event) error

func (f ObserverFunc) Notify(ctx context.Context, evt Event) error {
	if f == nil {
		return nil
	}
	return f(ctx, evt)
}

type Publisher interface {
	Publish(context.Context, Event)
}

type Subject struct {
	mu        sync.RWMutex
	observers []Observer
	onError   func(error)
}

func NewSubject(observers ...Observer) *Subject {
	return &Subject{observers: append([]Observer(nil), observers...)}
}

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

func (s *Subject) SetErrorHandler(fn func(error)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onError = fn
	s.mu.Unlock()
}
