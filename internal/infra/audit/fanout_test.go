package audit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type fakeSink struct {
	count int32
	err   error
}

func (s *fakeSink) Publish(_ context.Context, _ domain.AuditEvent) error {
	atomic.AddInt32(&s.count, 1)
	return s.err
}

func TestFanout_Publish_Success(t *testing.T) {
	s1 := &fakeSink{}
	s2 := &fakeSink{}
	f := NewFanout(s1, s2)

	evt := domain.AuditEvent{Timestamp: 1, Metrics: []string{"A"}, IPAddress: "1.1.1.1"}
	if err := f.Publish(context.Background(), evt); err != nil {
		t.Fatalf("Publish error: %v", err)
	}
	if got := atomic.LoadInt32(&s1.count); got != 1 {
		t.Fatalf("sink1 count=%d want 1", got)
	}
	if got := atomic.LoadInt32(&s2.count); got != 1 {
		t.Fatalf("sink2 count=%d want 1", got)
	}
}

func TestFanout_Publish_ErrorHandlerAndReturn(t *testing.T) {
	wantErr := errors.New("boom")
	s1 := &fakeSink{err: wantErr}
	s2 := &fakeSink{}
	f := NewFanout(s1, s2)

	var errCalls int32
	f.SetErrorHandler(func(err error) {
		if !errors.Is(err, wantErr) {
			t.Fatalf("unexpected error: %v", err)
		}
		atomic.AddInt32(&errCalls, 1)
	})

	err := f.Publish(context.Background(), domain.AuditEvent{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Publish error=%v want %v", err, wantErr)
	}
	if got := atomic.LoadInt32(&errCalls); got != 1 {
		t.Fatalf("errCalls=%d want 1", got)
	}
	if got := atomic.LoadInt32(&s2.count); got != 1 {
		t.Fatalf("sink2 count=%d want 1", got)
	}
}

func TestFanout_Attach_IgnoresNil(t *testing.T) {
	var nilSink ports.AuditPublisher
	s1 := &fakeSink{}
	f := NewFanout()
	f.Attach(nilSink, s1)

	if err := f.Publish(context.Background(), domain.AuditEvent{}); err != nil {
		t.Fatalf("Publish error: %v", err)
	}
	if got := atomic.LoadInt32(&s1.count); got != 1 {
		t.Fatalf("sink count=%d want 1", got)
	}
}

func TestFanout_NilReceiver(t *testing.T) {
	var f *Fanout
	if err := f.Publish(context.Background(), domain.AuditEvent{}); err != nil {
		t.Fatalf("Publish error: %v", err)
	}
	f.Attach(nil)
	f.SetErrorHandler(nil)
}

func TestFanout_MultipleErrors(t *testing.T) {
	wantErr := errors.New("boom")
	s1 := &fakeSink{err: wantErr}
	s2 := &fakeSink{err: wantErr}
	f := NewFanout(s1, s2)

	err := f.Publish(context.Background(), domain.AuditEvent{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Publish error=%v want %v", err, wantErr)
	}
	if got := atomic.LoadInt32(&s1.count); got != 1 {
		t.Fatalf("sink1 count=%d want 1", got)
	}
	if got := atomic.LoadInt32(&s2.count); got != 1 {
		t.Fatalf("sink2 count=%d want 1", got)
	}
}
