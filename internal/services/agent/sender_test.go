package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
)

type concPublisher struct {
	sleep     time.Duration
	failBatch bool

	mu          sync.Mutex
	inflight    int
	maxInflight int
	batchCalls  int
	singleCalls int
}

func (p *concPublisher) enter() {
	p.mu.Lock()
	p.inflight++
	if p.inflight > p.maxInflight {
		p.maxInflight = p.inflight
	}
	p.mu.Unlock()
}

func (p *concPublisher) leave() {
	p.mu.Lock()
	p.inflight--
	p.mu.Unlock()
}

func (p *concPublisher) SendBatch(ctx context.Context, items []domain.Metrics) error {
	p.enter()
	defer p.leave()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.sleep):
	}
	p.mu.Lock()
	p.batchCalls++
	p.mu.Unlock()
	if p.failBatch {
		return errors.New("batch failed")
	}
	return nil
}

func (p *concPublisher) SendOne(ctx context.Context, _ domain.Metrics) error {
	p.enter()
	defer p.leave()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.sleep):
	}
	p.mu.Lock()
	p.singleCalls++
	p.mu.Unlock()
	return nil
}

func mkBatch(n int) []domain.Metrics {
	out := make([]domain.Metrics, 0, n)
	for i := range n {
		v := float64(i)
		out = append(out, domain.Metrics{
			ID:    "g.test." + time.Now().Format("150405.000") + "." + string(rune('a'+i%26)),
			MType: string(domain.Gauge),
			Value: &v,
		})
	}
	return out
}

func TestBatchPublisher_RespectsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	pub := &concPublisher{sleep: 40 * time.Millisecond}
	bp := NewBatchPublisher(pub, 2)

	ctx := t.Context()

	bp.Start(ctx)

	for range 5 {
		bp.Submit(mkBatch(3))
	}

	bp.Stop()

	if pub.batchCalls != 5 {
		t.Fatalf("batchCalls=%d want=5", pub.batchCalls)
	}
	if pub.singleCalls != 0 {
		t.Fatalf("singleCalls=%d want=0", pub.singleCalls)
	}
	if pub.maxInflight != 2 {
		t.Fatalf("max inflight=%d, want=2", pub.maxInflight)
	}
}

func TestBatchPublisher_FallbackToSingles(t *testing.T) {
	t.Parallel()

	pub := &concPublisher{sleep: 10 * time.Millisecond, failBatch: true}
	bp := NewBatchPublisher(pub, 1)

	ctx := t.Context()

	bp.Start(ctx)

	const size = 7
	bp.Submit(mkBatch(size))

	bp.Stop()

	if pub.batchCalls != 1 {
		t.Fatalf("batchCalls=%d want=1", pub.batchCalls)
	}
	if pub.singleCalls != size {
		t.Fatalf("singleCalls=%d want=%d", pub.singleCalls, size)
	}
	if pub.maxInflight != 1 {
		t.Fatalf("max inflight=%d want=1", pub.maxInflight)
	}
}
