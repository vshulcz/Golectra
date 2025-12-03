package observer_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/vshulcz/Golectra/pkg/observer"
)

type testEvent struct {
	ID string
}

func TestSubject_Publish_NotifiesAll(t *testing.T) {
	subj := observer.NewSubject[testEvent]()
	var mu sync.Mutex
	var called []testEvent

	subj.Attach(observer.ObserverFunc[testEvent](func(_ context.Context, evt testEvent) error {
		mu.Lock()
		defer mu.Unlock()
		called = append(called, evt)
		return nil
	}))

	evt := testEvent{ID: "Alloc"}
	subj.Publish(context.Background(), evt)

	mu.Lock()
	defer mu.Unlock()
	if len(called) != 1 {
		t.Fatalf("expected 1 call, got %d", len(called))
	}
	if called[0].ID != evt.ID {
		t.Fatalf("event mismatch: %+v", called[0])
	}
}

func TestSubject_ErrorHandler(t *testing.T) {
	subj := observer.NewSubject[testEvent]()
	var mu sync.Mutex
	var errs []error

	subj.SetErrorHandler(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	})

	subj.Attach(observer.ObserverFunc[testEvent](func(_ context.Context, _ testEvent) error {
		return errors.New("boom")
	}))

	subj.Publish(context.Background(), testEvent{})

	mu.Lock()
	defer mu.Unlock()
	if len(errs) != 1 || errs[0].Error() != "boom" {
		t.Fatalf("expected error handler to capture boom, got %+v", errs)
	}
}
