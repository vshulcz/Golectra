package metrics

import (
	"context"
	"errors"
	"maps"
	"sync"
	"testing"

	"github.com/vshulcz/Golectra/internal/domain"
)

type fakeRepo struct {
	mu       sync.Mutex
	gauges   map[string]float64
	counters map[string]int64

	setGaugeCalls []struct {
		name  string
		value float64
	}
	addCounterCalls []struct {
		name  string
		delta int64
	}
	updateManyCalls [][]domain.Metrics
	pingCalls       int
	snapshotCalls   int

	pingErr         error
	setGaugeErr     map[string]error
	addCounterErr   map[string]error
	updateManyErr   error
	nextSnapshotErr error
	getGaugeErr     map[string]error
	getCounterErr   map[string]error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		gauges:        map[string]float64{},
		counters:      map[string]int64{},
		setGaugeErr:   map[string]error{},
		addCounterErr: map[string]error{},
		getGaugeErr:   map[string]error{},
		getCounterErr: map[string]error{},
	}
}

func (r *fakeRepo) GetGauge(_ context.Context, name string) (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.getGaugeErr[name]; err != nil {
		return 0, err
	}
	v, ok := r.gauges[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *fakeRepo) GetCounter(_ context.Context, name string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.getCounterErr[name]; err != nil {
		return 0, err
	}
	v, ok := r.counters[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *fakeRepo) SetGauge(_ context.Context, name string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.setGaugeErr[name]; err != nil {
		return err
	}
	r.setGaugeCalls = append(r.setGaugeCalls, struct {
		name  string
		value float64
	}{name, value})
	r.gauges[name] = value
	return nil
}

func (r *fakeRepo) AddCounter(_ context.Context, name string, delta int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.addCounterErr[name]; err != nil {
		return err
	}
	r.addCounterCalls = append(r.addCounterCalls, struct {
		name  string
		delta int64
	}{name, delta})
	r.counters[name] += delta
	return nil
}

func (r *fakeRepo) UpdateMany(_ context.Context, items []domain.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateManyErr != nil {
		return r.updateManyErr
	}
	r.updateManyCalls = append(r.updateManyCalls, append([]domain.Metrics(nil), items...))
	for _, it := range items {
		switch it.MType {
		case string(domain.Gauge):
			if it.Value != nil {
				r.gauges[it.ID] = *it.Value
			}
		case string(domain.Counter):
			if it.Delta != nil {
				r.counters[it.ID] += *it.Delta
			}
		}
	}
	return nil
}

func (r *fakeRepo) Snapshot(_ context.Context) (domain.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshotCalls++
	if r.nextSnapshotErr != nil {
		err := r.nextSnapshotErr
		r.nextSnapshotErr = nil
		return domain.Snapshot{Gauges: map[string]float64{}, Counters: map[string]int64{}}, err
	}
	g := make(map[string]float64, len(r.gauges))
	maps.Copy(g, r.gauges)
	c := make(map[string]int64, len(r.counters))
	maps.Copy(c, r.counters)
	return domain.Snapshot{Gauges: g, Counters: c}, nil
}

func (r *fakeRepo) Ping(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pingCalls++
	return r.pingErr
}

func TestService_Ping_Error(t *testing.T) {
	repo := newFakeRepo()
	s := New(repo, nil)

	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping unexpected err: %v", err)
	}
	repo.pingErr = errors.New("down")
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("expected error from Ping")
	}
}

func TestService_Get(t *testing.T) {
	repo := newFakeRepo()
	repo.gauges["Alloc"] = 1.25
	repo.counters["PollCount"] = 9

	svc := New(repo, nil)

	tests := []struct {
		name    string
		typ     string
		id      string
		setup   func()
		wantOK  bool
		wantV   float64
		wantD   int64
		wantErr error
	}{
		{"empty_id_trimmed", string(domain.Gauge), "   ", nil, false, 0, 0, domain.ErrNotFound},
		{"invalid_type", "weird", "x", nil, false, 0, 0, domain.ErrInvalidType},
		{"gauge_ok", string(domain.Gauge), "Alloc", nil, true, 1.25, 0, nil},
		{"gauge_not_found", string(domain.Gauge), "Missing", nil, false, 0, 0, domain.ErrNotFound},
		{"counter_ok", string(domain.Counter), "PollCount", nil, true, 0, 9, nil},
		{"counter_not_found", string(domain.Counter), "Nope", nil, false, 0, 0, domain.ErrNotFound},
		{"gauge_repo_generic_error", string(domain.Gauge), "Boom", func() { repo.getGaugeErr["Boom"] = errors.New("boom") }, false, 0, 0, errors.New("boom")},
		{"counter_repo_generic_error", string(domain.Counter), "BoomC", func() { repo.getCounterErr["BoomC"] = errors.New("xx") }, false, 0, 0, errors.New("xx")},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			m, err := svc.Get(context.Background(), tc.typ, tc.id)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err=%v wantOK=%v", err, tc.wantOK)
			}
			if tc.wantErr != nil && (err == nil || err.Error() != tc.wantErr.Error()) {
				t.Fatalf("err=%v want %v", err, tc.wantErr)
			}
			if tc.wantOK {
				if m.ID != stringsTrim(tc.id) || m.MType != tc.typ {
					t.Fatalf("returned %+v mismatch id/type", m)
				}
				if tc.typ == string(domain.Gauge) {
					if m.Value == nil || *m.Value != tc.wantV || m.Delta != nil {
						t.Fatalf("gauge payload wrong: %+v want %v", m, tc.wantV)
					}
				} else {
					if m.Delta == nil || *m.Delta != tc.wantD || m.Value != nil {
						t.Fatalf("counter payload wrong: %+v want %v", m, tc.wantD)
					}
				}
			}
		})
	}
}

func stringsTrim(s string) string {
	return s
}

func TestService_Upsert(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, nil)

	tests := []struct {
		name    string
		m       domain.Metrics
		setup   func()
		wantOK  bool
		wantErr error
	}{
		{"empty_id", domain.Metrics{ID: "   ", MType: string(domain.Gauge), Value: ptrFloat64(1.0)}, nil, false, domain.ErrNotFound},
		{"gauge_nil_value", domain.Metrics{ID: "A", MType: string(domain.Gauge), Value: nil}, nil, false, domain.ErrInvalidType},
		{"counter_nil_delta", domain.Metrics{ID: "C", MType: string(domain.Counter), Delta: nil}, nil, false, domain.ErrInvalidType},
		{"invalid_type", domain.Metrics{ID: "X", MType: "unknown", Value: ptrFloat64(1)}, nil, false, domain.ErrInvalidType},
		{"gauge_set_error", domain.Metrics{ID: "GErr", MType: string(domain.Gauge), Value: ptrFloat64(2.2)}, func() { repo.setGaugeErr["GErr"] = errors.New("sg") }, false, errors.New("sg")},
		{"counter_add_error", domain.Metrics{ID: "CErr", MType: string(domain.Counter), Delta: ptrInt(2)}, func() { repo.addCounterErr["CErr"] = errors.New("ac") }, false, errors.New("ac")},
		{"gauge_ok", domain.Metrics{ID: "Alloc", MType: string(domain.Gauge), Value: ptrFloat64(3.14)}, nil, true, nil},
		{"counter_ok", domain.Metrics{ID: "Poll", MType: string(domain.Counter), Delta: ptrInt(5)}, nil, true, nil},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			got, err := svc.Upsert(context.Background(), tc.m)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err=%v wantOK=%v", err, tc.wantOK)
			}
			if tc.wantErr != nil && (err == nil || err.Error() != tc.wantErr.Error()) {
				t.Fatalf("err=%v want %v", err, tc.wantErr)
			}
			if tc.wantOK {
				switch tc.m.MType {
				case string(domain.Gauge):
					if v, ok := repo.gauges[tc.m.ID]; !ok || v != *tc.m.Value {
						t.Fatalf("repo gauge not updated: got %v want %v", v, *tc.m.Value)
					}
					if got.Value == nil || *got.Value != *tc.m.Value || got.MType != string(domain.Gauge) {
						t.Fatalf("returned %+v mismatch gauge", got)
					}
				case string(domain.Counter):
					if v, ok := repo.counters[tc.m.ID]; !ok || v == 0 {
						t.Fatalf("repo counter not updated: %v", v)
					}
					if got.Delta == nil || *got.Delta != repo.counters[tc.m.ID] || got.MType != string(domain.Counter) {
						t.Fatalf("returned %+v mismatch counter", got)
					}
				}
			}
		})
	}
}

func TestService_UpsertBatch(t *testing.T) {
	repo := newFakeRepo()
	repo.gauges["A"] = 1.0
	repo.counters["C"] = 2

	var cbMu sync.Mutex
	var cbCalls int
	var cbLast domain.Snapshot
	cb := func(_ context.Context, s domain.Snapshot) {
		cbMu.Lock()
		defer cbMu.Unlock()
		cbCalls++
		cbLast = s
	}

	svc := New(repo, cb)

	validGauge := domain.Metrics{ID: "g", MType: string(domain.Gauge), Value: ptrFloat64(1.5)}
	validCounter := domain.Metrics{ID: "c", MType: string(domain.Counter), Delta: ptrInt(3)}
	invalids := []domain.Metrics{
		{ID: "", MType: string(domain.Gauge), Value: ptrFloat64(1)},
		{ID: "x", MType: string(domain.Gauge), Value: nil},
		{ID: "y", MType: string(domain.Counter), Delta: nil},
		{ID: "z", MType: "weird", Value: ptrFloat64(1)},
	}

	t.Run("all_invalid", func(t *testing.T) {
		repo.updateManyErr = nil
		cbMu.Lock()
		cbCalls = 0
		cbMu.Unlock()

		n, err := svc.UpsertBatch(context.Background(), invalids)
		if n != 0 || !errors.Is(err, domain.ErrInvalidType) {
			t.Fatalf("n=%d err=%v want 0, ErrInvalidType", n, err)
		}
		if len(repo.updateManyCalls) != 0 {
			t.Fatalf("UpdateMany should not be called")
		}
		if cbCalls != 0 {
			t.Fatalf("onChanged should not be called")
		}
	})

	t.Run("mixed_valid_invalid_update_ok_triggers_onChanged", func(t *testing.T) {
		repo.updateManyErr = nil
		cbMu.Lock()
		cbCalls = 0
		cbMu.Unlock()
		repo.nextSnapshotErr = nil

		in := append([]domain.Metrics{validGauge, validCounter}, invalids...)
		n, err := svc.UpsertBatch(context.Background(), in)
		if err != nil || n != 2 {
			t.Fatalf("n=%d err=%v want 2, nil", n, err)
		}
		if len(repo.updateManyCalls) != 1 {
			t.Fatalf("UpdateMany called %d want 1", len(repo.updateManyCalls))
		}
		cbMu.Lock()
		defer cbMu.Unlock()
		if cbCalls != 1 {
			t.Fatalf("onChanged calls=%d want 1", cbCalls)
		}
		if cbLast.Gauges["A"] != 1.0 || cbLast.Counters["C"] != 2 {
			t.Fatalf("onChanged snapshot mismatch: %+v", cbLast)
		}
	})

	t.Run("update_err_propagates_and_no_onChanged", func(t *testing.T) {
		repo.updateManyErr = errors.New("fail")
		cbMu.Lock()
		cbCalls = 0
		cbMu.Unlock()

		n, err := svc.UpsertBatch(context.Background(), []domain.Metrics{validGauge})
		if n != 0 || err == nil || err.Error() != "fail" {
			t.Fatalf("n=%d err=%v want 0, fail", n, err)
		}
		if cbCalls != 0 {
			t.Fatalf("onChanged should not be called")
		}
	})

	t.Run("snapshot_err_suppressed_on_onChanged", func(t *testing.T) {
		repo.updateManyErr = nil
		repo.nextSnapshotErr = errors.New("snap-err")
		cbMu.Lock()
		cbCalls = 0
		cbMu.Unlock()

		n, err := svc.UpsertBatch(context.Background(), []domain.Metrics{validCounter})
		if err != nil || n != 1 {
			t.Fatalf("n=%d err=%v want 1, nil", n, err)
		}
		cbMu.Lock()
		defer cbMu.Unlock()
		if cbCalls != 0 {
			t.Fatalf("onChanged should not be called when snapshot fails")
		}
	})
}

func TestService_Snapshot_Proxy(t *testing.T) {
	repo := newFakeRepo()
	repo.gauges["g"] = 2.2
	repo.counters["c"] = 8
	svc := New(repo, nil)

	s, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot err: %v", err)
	}
	if s.Gauges["g"] != 2.2 || s.Counters["c"] != 8 {
		t.Fatalf("snapshot mismatch: %+v", s)
	}

	repo.nextSnapshotErr = errors.New("boom")
	if _, err := svc.Snapshot(context.Background()); err == nil {
		t.Fatal("expected snapshot error")
	}
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt(v int64) *int64         { return &v }
