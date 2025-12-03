package audit

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestSubject_Publish_NotifiesAll(t *testing.T) {
	s := NewSubject()
	var mu sync.Mutex
	var called []Event

	s.Attach(ObserverFunc(func(_ context.Context, evt Event) error {
		mu.Lock()
		defer mu.Unlock()
		called = append(called, evt)
		return nil
	}))

	evt := Event{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "1.1.1.1"}
	s.Publish(context.Background(), evt)

	mu.Lock()
	defer mu.Unlock()
	if len(called) != 1 {
		t.Fatalf("expected 1 call, got %d", len(called))
	}
	if called[0].IPAddress != evt.IPAddress {
		t.Fatalf("event mismatch: %+v", called[0])
	}
}

func TestSubject_ErrorHandler(t *testing.T) {
	s := NewSubject()
	var mu sync.Mutex
	var errs []error

	s.SetErrorHandler(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	})

	s.Attach(ObserverFunc(func(_ context.Context, _ Event) error {
		return errors.New("boom")
	}))

	s.Publish(context.Background(), Event{})

	mu.Lock()
	defer mu.Unlock()
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Fatalf("expected error handler to capture boom, got %+v", errs)
	}
}
