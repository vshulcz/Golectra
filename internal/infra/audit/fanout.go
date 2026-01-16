// Package audit provides infrastructure helpers for audit fan-out.
package audit

import (
	"context"
	"sync"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

// Fanout publishes audit events to multiple sinks.
type Fanout struct {
	mu      sync.RWMutex
	sinks   []ports.AuditPublisher
	onError func(error)
}

// NewFanout creates a fanout publisher with optional initial sinks.
func NewFanout(sinks ...ports.AuditPublisher) *Fanout {
	cp := append([]ports.AuditPublisher(nil), sinks...)
	return &Fanout{sinks: cp}
}

// Publish forwards the event to all registered sinks.
func (f *Fanout) Publish(ctx context.Context, evt domain.AuditEvent) error {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	sinks := append([]ports.AuditPublisher(nil), f.sinks...)
	errHandler := f.onError
	f.mu.RUnlock()

	var firstErr error
	for _, sink := range sinks {
		if sink == nil {
			continue
		}
		if err := sink.Publish(ctx, evt); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			if errHandler != nil {
				errHandler(err)
			}
		}
	}
	return firstErr
}

// Attach registers additional audit sinks.
func (f *Fanout) Attach(sinks ...ports.AuditPublisher) {
	if f == nil || len(sinks) == 0 {
		return
	}
	f.mu.Lock()
	f.sinks = append(f.sinks, sinks...)
	f.mu.Unlock()
}

// SetErrorHandler configures a callback for sink failures.
func (f *Fanout) SetErrorHandler(fn func(error)) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.onError = fn
	f.mu.Unlock()
}
