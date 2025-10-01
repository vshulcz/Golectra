package agent

import (
	"context"
	"errors"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
)

type fakeCollector struct {
	mu       sync.RWMutex
	started  bool
	startErr error
	stopped  bool
	interval time.Duration
	gauges   map[string]float64
	counters map[string]int64
}

func (f *fakeCollector) Start(_ context.Context, interval time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	f.interval = interval
	return f.startErr
}

func (f *fakeCollector) Stop() {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
}

func (f *fakeCollector) Snapshot() (map[string]float64, map[string]int64) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	g := make(map[string]float64, len(f.gauges))
	maps.Copy(g, f.gauges)
	c := make(map[string]int64, len(f.counters))
	maps.Copy(c, f.counters)
	return g, c
}

type fakePublisher struct {
	mu         sync.Mutex
	batchCalls int
	lastBatch  []domain.Metrics
	batchErr   error

	singleCalls []domain.Metrics
	singleErrs  map[string]error
}

func (p *fakePublisher) SendBatch(_ context.Context, items []domain.Metrics) error {
	p.mu.Lock()
	p.batchCalls++
	p.lastBatch = append([]domain.Metrics(nil), items...)
	err := p.batchErr
	p.mu.Unlock()
	return err
}

func (p *fakePublisher) SendOne(_ context.Context, item domain.Metrics) error {
	p.mu.Lock()
	p.singleCalls = append(p.singleCalls, item)
	var err error
	if p.singleErrs != nil {
		err = p.singleErrs[item.ID]
	}
	p.mu.Unlock()
	return err
}

func TestService_reportOnce(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name        string
		gauges      map[string]float64
		counters    map[string]int64
		batchErr    error
		singleErrs  map[string]error
		wantBatch   int
		wantSingles int
		wantIDs     map[string]string
	}
	tests := []testcase{
		{
			name:        "empty_snapshot_no_publish",
			gauges:      map[string]float64{},
			counters:    map[string]int64{},
			wantBatch:   0,
			wantSingles: 0,
		},
		{
			name:        "batch_ok_one_gauge_one_counter",
			gauges:      map[string]float64{"Alloc": 1.23},
			counters:    map[string]int64{"PollCount": 7},
			wantBatch:   1,
			wantSingles: 0,
			wantIDs:     map[string]string{"Alloc": "gauge", "PollCount": "counter"},
		},
		{
			name:        "batch_fail_fallback_all_single_ok",
			gauges:      map[string]float64{"G1": 10},
			counters:    map[string]int64{"C1": 5},
			batchErr:    errors.New("boom"),
			wantBatch:   1,
			wantSingles: 2,
			wantIDs:     map[string]string{"G1": "gauge", "C1": "counter"},
		},
		{
			name:     "batch_fail_fallback_some_single_fail",
			gauges:   map[string]float64{"G": 2.5},
			counters: map[string]int64{"C": 9},
			batchErr: errors.New("down"),
			singleErrs: map[string]error{
				"G": errors.New("single-g-err"),
			},
			wantBatch:   1,
			wantSingles: 2,
			wantIDs:     map[string]string{"G": "gauge", "C": "counter"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			coll := &fakeCollector{gauges: tc.gauges, counters: tc.counters}
			pub := &fakePublisher{batchErr: tc.batchErr, singleErrs: tc.singleErrs}
			svc := &Service{collector: coll, pub: pub}

			svc.reportOnce(context.Background())

			if pub.batchCalls != tc.wantBatch {
				t.Fatalf("batchCalls=%d want %d", pub.batchCalls, tc.wantBatch)
			}
			if len(pub.singleCalls) != tc.wantSingles {
				t.Fatalf("singleCalls=%d want %d", len(pub.singleCalls), tc.wantSingles)
			}

			if tc.wantBatch > 0 {
				got := indexByID(pub.lastBatch)
				for id, typ := range tc.wantIDs {
					m, ok := got[id]
					if !ok {
						t.Fatalf("batch missing id=%q", id)
					}
					if m.MType != typ {
						t.Fatalf("batch id=%q type=%q want %q", id, m.MType, typ)
					}
					switch typ {
					case "gauge":
						if m.Value == nil || m.Delta != nil {
							t.Fatalf("gauge id=%q payload mismatch: %+v", id, m)
						}
					case "counter":
						if m.Delta == nil || m.Value != nil {
							t.Fatalf("counter id=%q payload mismatch: %+v", id, m)
						}
					default:
					}
				}
			}

			if tc.wantSingles > 0 {
				got := indexByID(pub.singleCalls)
				for id, typ := range tc.wantIDs {
					m, ok := got[id]
					if !ok {
						t.Fatalf("single missing id=%q", id)
					}
					if m.MType != typ {
						t.Fatalf("single id=%q type=%q want %q", id, m.MType, typ)
					}
				}
			}
		})
	}
}

func indexByID(items []domain.Metrics) map[string]domain.Metrics {
	out := make(map[string]domain.Metrics, len(items))
	for _, m := range items {
		out[m.ID] = m
	}
	return out
}

func TestService_Run_Error(t *testing.T) {
	coll := &fakeCollector{startErr: errors.New("nope")}
	pub := &fakePublisher{}
	cfg := config.AgentConfig{PollInterval: 1 * time.Millisecond, ReportInterval: 2 * time.Millisecond}

	svc := New(cfg, coll, pub)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := svc.Run(ctx); err == nil {
		t.Fatal("expected start error")
	}
	if coll.stopped {
		t.Fatal("collector.Stop() must not be called on start error")
	}
}

func TestService_Run(t *testing.T) {
	coll := &fakeCollector{
		gauges:   map[string]float64{"Alloc": 1.0},
		counters: map[string]int64{"PollCount": 1},
	}
	pub := &fakePublisher{}

	cfg := config.AgentConfig{
		PollInterval:   1 * time.Millisecond,
		ReportInterval: 5 * time.Millisecond,
	}
	svc := New(cfg, coll, pub)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- svc.Run(ctx) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run did not exit after cancel")
	}

	if !coll.started {
		t.Fatal("collector.Start was not called")
	}
	if !coll.stopped {
		t.Fatal("collector.Stop was not called")
	}
	if pub.batchCalls == 0 {
		t.Fatal("expected at least one batch publish")
	}
}

func TestService_Run_EmptySnapshot(t *testing.T) {
	coll := &fakeCollector{
		gauges:   map[string]float64{},
		counters: map[string]int64{},
	}
	pub := &fakePublisher{}
	cfg := config.AgentConfig{PollInterval: 1 * time.Millisecond, ReportInterval: 5 * time.Millisecond}
	svc := New(cfg, coll, pub)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- svc.Run(ctx) }()

	time.Sleep(15 * time.Millisecond)
	cancel()

	<-errCh

	if pub.batchCalls != 0 || len(pub.singleCalls) != 0 {
		t.Fatalf("publishes happened unexpectedly: batch=%d singles=%d", pub.batchCalls, len(pub.singleCalls))
	}
}
