package memory

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/vshulcz/Golectra/internal/domain"
)

func TestMemStorage(t *testing.T) {
	t.Run("UpdateAndGetGauge", func(t *testing.T) {
		ms := New()
		cases := []struct {
			name   string
			key    string
			value  float64
			expect float64
		}{
			{"first set", "Alloc", 123.45, 123.45},
			{"overwrite value", "Alloc", 99.9, 99.9},
			{"new key", "Heap", 42.0, 42.0},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := ms.SetGauge(context.TODO(), tc.key, tc.value); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got, err := ms.GetGauge(context.TODO(), tc.key)
				if err != nil {
					t.Fatalf("expected gauge %q to exist", tc.key)
				}
				if got != tc.expect {
					t.Errorf("got %v, want %v", got, tc.expect)
				}
			})
		}
	})

	t.Run("UpdateAndGetCounter", func(t *testing.T) {
		ms := New()
		cases := []struct {
			name     string
			key      string
			deltas   []int64
			expected int64
		}{
			{"single increment", "PollCount", []int64{1}, 1},
			{"multiple increments", "PollCount", []int64{2, 3}, 6},
			{"independent keys", "Another", []int64{10}, 10},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				for _, d := range tc.deltas {
					if err := ms.AddCounter(context.TODO(), tc.key, d); err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				}
				got, err := ms.GetCounter(context.TODO(), tc.key)
				if err != nil {
					t.Fatalf("expected counter %q to exist", tc.key)
				}
				if got != tc.expected {
					t.Errorf("got %v, want %v", got, tc.expected)
				}
			})
		}
	})

	t.Run("GetNonexistent", func(t *testing.T) {
		ms := New()
		if _, err := ms.GetGauge(context.TODO(), "missing"); err == nil {
			t.Error("expected GetGauge to return err for missing key")
		}
		if _, err := ms.GetCounter(context.TODO(), "missing"); err == nil {
			t.Error("expected GetCounter to return err for missing key")
		}
	})

	t.Run("UpdateMany", func(t *testing.T) {
		type want struct {
			g map[string]float64
			c map[string]int64
		}
		cases := []struct {
			want  want
			name  string
			items []domain.Metrics
		}{
			{
				name: "mixed with overwrite/sum and ignored items",
				items: []domain.Metrics{
					{ID: "g1", MType: string(domain.Gauge), Value: ptrFloat64(3.14)},
					{ID: "c1", MType: string(domain.Counter), Delta: ptrInt64(5)},
					{ID: "g1", MType: string(domain.Gauge), Value: ptrFloat64(2.71)},
					{ID: "c1", MType: string(domain.Counter), Delta: ptrInt64(7)},
					{ID: "nullg", MType: string(domain.Gauge), Value: nil},
					{ID: "nullc", MType: string(domain.Counter), Delta: nil},
					{ID: "bad", MType: "weird", Value: ptrFloat64(1)},
				},
				want: want{
					g: map[string]float64{"g1": 2.71},
					c: map[string]int64{"c1": 12},
				},
			},
			{
				name:  "empty slice is noop",
				items: []domain.Metrics{},
				want:  want{g: map[string]float64{}, c: map[string]int64{}},
			},
			{
				name:  "nil slice is noop",
				items: nil,
				want:  want{g: map[string]float64{}, c: map[string]int64{}},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ms := New()
				ms.SetGauge(context.TODO(), "pre_g", 10)
				ms.AddCounter(context.TODO(), "pre_c", 5)

				if err := ms.UpdateMany(context.TODO(), tc.items); err != nil {
					t.Fatalf("UpdateMany error: %v", err)
				}

				for k, v := range tc.want.g {
					if got, err := ms.GetGauge(context.TODO(), k); err != nil || got != v {
						t.Fatalf("gauge %q=%v err=%v, want %v true", k, got, err, v)
					}
				}
				for k, v := range tc.want.c {
					if got, err := ms.GetCounter(context.TODO(), k); err != nil || got != v {
						t.Fatalf("counter %q=%v err=%v, want %v true", k, got, err, v)
					}
				}
				if _, err := ms.GetGauge(context.TODO(), "nullg"); err == nil {
					t.Fatal("nullg must not be created")
				}
				if _, err := ms.GetCounter(context.TODO(), "nullc"); err == nil {
					t.Fatal("nullc must not be created")
				}
				if _, err := ms.GetGauge(context.TODO(), "bad"); err == nil {
					t.Fatal("bad should be ignored (unknown type)")
				}

				if v, err := ms.GetGauge(context.TODO(), "pre_g"); err != nil || v != 10 {
					t.Fatalf("pre_g changed: %v err=%v", v, err)
				}
				if d, err := ms.GetCounter(context.TODO(), "pre_c"); err != nil || d != 5 {
					t.Fatalf("pre_c changed: %v err=%v", d, err)
				}
			})
		}
	})

	t.Run("SnapshotIsolation", func(t *testing.T) {
		ms := New()
		ms.SetGauge(context.TODO(), "g", 1.0)
		ms.AddCounter(context.TODO(), "c", 1)

		s, _ := ms.Snapshot(context.TODO())
		s.Gauges["g"] = 100.0
		s.Counters["c"] = 100

		if v, _ := ms.GetGauge(context.TODO(), "g"); v != 1.0 {
			t.Fatalf("snapshot must be a copy for gauges, got %v", v)
		}
		if d, _ := ms.GetCounter(context.TODO(), "c"); d != 1 {
			t.Fatalf("snapshot must be a copy for counters, got %v", d)
		}
	})

	t.Run("PingNotConfigured", func(t *testing.T) {
		ms := New()
		if err := ms.Ping(context.TODO()); err == nil {
			t.Fatal("Ping must return error when DB not configured")
		}
	})
	t.Run("Concurrent", func(t *testing.T) {
		t.Run("BasicAccess", func(_ *testing.T) {
			ms := New()
			var wg sync.WaitGroup

			for i := range 10 {
				wg.Add(1)
				func(i int) {
					defer wg.Done()
					ms.SetGauge(context.TODO(), "g", float64(i))
					ms.AddCounter(context.TODO(), "c", int64(i))
				}(i)
			}
			for range 10 {
				wg.Go(func() {
					ms.GetGauge(context.TODO(), "g")
					ms.GetCounter(context.TODO(), "c")
				})
			}
			wg.Wait()
		})

		t.Run("UpdateManyConcurrentCounters", func(t *testing.T) {
			ms := New()
			const workers = 16
			const perWorker = 250

			var wg sync.WaitGroup
			wg.Add(workers)
			for range workers {
				go func() {
					defer wg.Done()
					items := make([]domain.Metrics, 0, perWorker)
					for range perWorker {
						d := int64(1)
						items = append(items, domain.Metrics{ID: "c", MType: string(domain.Counter), Delta: &d})
					}
					ms.UpdateMany(context.TODO(), items)
				}()
			}
			wg.Wait()

			want := int64(workers * perWorker)
			if got, err := ms.GetCounter(context.TODO(), "c"); err != nil || got != want {
				t.Fatalf("counter c=%v err=%v, want %v true", got, err, want)
			}
		})

		t.Run("UpdateManyConcurrentGauge", func(t *testing.T) {
			ms := New()
			const workers = 8
			var wg sync.WaitGroup
			wg.Add(workers)

			values := make([]float64, workers)
			for i := range workers {
				values[i] = float64(i)
			}

			for i := range workers {
				go func() {
					defer wg.Done()
					items := []domain.Metrics{{ID: "g", MType: string(domain.Gauge), Value: ptrFloat64(values[i])}}
					ms.UpdateMany(context.TODO(), items)
				}()
			}
			wg.Wait()

			got, err := ms.GetGauge(context.TODO(), "g")
			if err != nil {
				t.Fatal("gauge g must exist")
			}
			found := slices.Contains(values, got)
			if !found {
				t.Fatalf("final gauge value %v is not one of %v", got, values)
			}
		})
	})
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }
