package state

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStats_Snapshot(t *testing.T) {
	s := New()
	g, c := s.Snapshot()
	if len(g) != 0 || len(c) != 0 {
		t.Fatalf("expected empty maps, got gauges=%v counters=%v", g, c)
	}
}

func TestStats_SetGauge(t *testing.T) {
	type step struct {
		key string
		val float64
	}
	tests := []struct {
		name string
		seq  []step
		want map[string]float64
	}{
		{
			name: "single_key_overwrite",
			seq: []step{
				{"g1", 1.0},
				{"g1", 2.5},
				{"g1", -7.75},
			},
			want: map[string]float64{"g1": -7.75},
		},
		{
			name: "multiple_keys",
			seq: []step{
				{"a", 10},
				{"b", 20},
				{"a", 30},
			},
			want: map[string]float64{"a": 30, "b": 20},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			for _, st := range tc.seq {
				s.SetGauge(st.key, st.val)
			}
			got, _ := s.Snapshot()
			if len(got) != len(tc.want) {
				t.Fatalf("len(gauges)=%d want %d, got=%v", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if gv, ok := got[k]; !ok || gv != v {
					t.Fatalf("gauge[%q]=%v want %v, all=%v", k, gv, v, got)
				}
			}
		})
	}
}

func TestStats_AddCounter(t *testing.T) {
	type step struct {
		key string
		d   int64
	}
	tests := []struct {
		name string
		seq  []step
		want map[string]int64
	}{
		{
			name: "single_key_accumulate",
			seq:  []step{{"c1", 3}, {"c1", 4}, {"c1", -2}},
			want: map[string]int64{"c1": 5},
		},
		{
			name: "multiple_keys",
			seq:  []step{{"x", 1}, {"y", 10}, {"x", 2}, {"y", -3}},
			want: map[string]int64{"x": 3, "y": 7},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			for _, st := range tc.seq {
				s.AddCounter(st.key, st.d)
			}
			_, got := s.Snapshot()
			if len(got) != len(tc.want) {
				t.Fatalf("len(counters)=%d want %d, got=%v", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if gv, ok := got[k]; !ok || gv != v {
					t.Fatalf("counter[%q]=%v want %v, all=%v", k, gv, v, got)
				}
			}
		})
	}
}

func TestStats_Snapshot_Independence(t *testing.T) {
	s := New()
	s.SetGauge("g", 1.23)
	s.AddCounter("c", 5)

	g1, c1 := s.Snapshot()

	g1["g"] = 999
	g1["new"] = 100
	c1["c"] = 999
	c1["new"] = 100

	g2, c2 := s.Snapshot()

	if v := g2["g"]; v != 1.23 {
		t.Fatalf("gauge 'g' mutated via snapshot: got=%v want=1.23", v)
	}
	if _, ok := g2["new"]; ok {
		t.Fatalf("unexpected gauge 'new' leaked into state")
	}
	if v := c2["c"]; v != 5 {
		t.Fatalf("counter 'c' mutated via snapshot: got=%v want=5", v)
	}
	if _, ok := c2["new"]; ok {
		t.Fatalf("unexpected counter 'new' leaked into state")
	}
}

func TestStats_ConcurrentCounterAdds(t *testing.T) {
	s := New()

	const (
		goroutines = 8
		perG       = 5000
		key        = "counter"
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range perG {
				s.AddCounter(key, 1)
			}
		}()
	}
	wg.Wait()

	_, c := s.Snapshot()
	got := c[key]
	want := int64(goroutines * perG)
	if got != want {
		t.Fatalf("counter total=%d want=%d", got, want)
	}
}

func TestStats_ConcurrentGaugeSet(t *testing.T) {
	s := New()

	const (
		goroutines = 16
		keysPerG   = 32
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func(gid int) {
			defer wg.Done()
			for i := range keysPerG {
				k := fmt.Sprintf("g_%02d_%02d", gid, i)
				s.SetGauge(k, float64(gid*keysPerG+i))
			}
		}(g)
	}
	wg.Wait()

	g, _ := s.Snapshot()
	wantLen := goroutines * keysPerG
	if len(g) != wantLen {
		t.Fatalf("len(gauges)=%d want=%d", len(g), wantLen)
	}

	for _, probe := range []struct {
		gid, idx int
	}{
		{0, 0}, {goroutines - 1, keysPerG - 1}, {goroutines / 2, keysPerG / 2},
	} {
		k := fmt.Sprintf("g_%02d_%02d", probe.gid, probe.idx)
		want := float64(probe.gid*keysPerG + probe.idx)
		if v, ok := g[k]; !ok || v != want {
			t.Fatalf("gauge[%q]=%v ok=%v want=%v", k, v, ok, want)
		}
	}
}

func TestStats_Snapshot_NoPanic_NoRace(t *testing.T) {
	s := New()
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s.SetGauge("g", float64(time.Now().UnixNano()))
				s.AddCounter("c", 1)
			}
		}
	}()

	for range 200 {
		_, _ = s.Snapshot()
		time.Sleep(500 * time.Microsecond)
	}
	close(stop)
}
