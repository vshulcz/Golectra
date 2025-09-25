package runtime

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

func waitForPollCount(p *Collector, want int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, cnt := p.Snapshot()
		if v, ok := cnt[MPollCount]; ok && v >= want {
			return true
		}
		time.Sleep(1 * time.Millisecond)
	}
	return false
}

func TestCollector_SetsMetricsAndRandomValue(t *testing.T) {
	type testCase struct {
		name       string
		ticks      int64
		interval   time.Duration
		requireAll bool
	}
	tests := []testCase{
		{
			name:       "one_tick_minimal_keys",
			ticks:      1,
			interval:   5 * time.Millisecond,
			requireAll: false,
		},
		{
			name:       "two_ticks_all_keys",
			ticks:      2,
			interval:   4 * time.Millisecond,
			requireAll: true,
		},
	}

	allKeys := []string{
		MAlloc, MBuckHashSys, MFrees, MGCCPUFraction, MGCSys,
		MHeapAlloc, MHeapIdle, MHeapInuse, MHeapObjects, MHeapReleased,
		MHeapSys, MLastGC, MLookups, MMCacheInuse, MMCacheSys,
		MMSpanInuse, MMSpanSys, MMallocs, MNextGC, MNumForcedGC,
		MNumGC, MOtherSys, MPauseTotalNs, MStackInuse, MStackSys,
		MSys, MTotalAlloc, MRandomValue,
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New()
			p.rnd = rand.New(rand.NewSource(1))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := p.Start(ctx, tc.interval); err != nil {
				t.Fatalf("Start error: %v", err)
			}

			if ok := waitForPollCount(p, tc.ticks, 500*time.Millisecond); !ok {
				p.Stop()
				t.Fatalf("timeout waiting for PollCount >= %d", tc.ticks)
			}

			p.Stop()
			time.Sleep(2 * tc.interval)

			g, cnt := p.Snapshot()
			gotPoll, ok := cnt[MPollCount]
			if !ok {
				t.Fatalf("PollCount not present")
			}
			if gotPoll < tc.ticks {
				t.Fatalf("PollCount=%d < expected ticks=%d", gotPoll, tc.ticks)
			}

			minKeys := []string{MAlloc, MHeapAlloc, MSys, MRandomValue}
			for _, k := range minKeys {
				if _, ok := g[k]; !ok {
					t.Fatalf("gauge %q not set", k)
				}
			}

			if rv, ok := g[MRandomValue]; !ok {
				t.Fatalf("RandomValue missing")
			} else if rv < 0.0 || rv >= 1.0 {
				t.Fatalf("RandomValue out of range [0,1): %v", rv)
			}

			if tc.requireAll {
				for _, k := range allKeys {
					if _, ok := g[k]; !ok {
						t.Fatalf("expected gauge %q to be set", k)
					}
				}
			}
		})
	}
}

func TestPoller_StopsAndNoFurtherIncrements(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		ticks    int64
	}{
		{"stop_after_3_ticks_5ms", 5 * time.Millisecond, 3},
		{"stop_after_5_ticks_2ms", 2 * time.Millisecond, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New()
			p.rnd = rand.New(rand.NewSource(2))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := p.Start(ctx, tc.interval); err != nil {
				t.Fatalf("Start error: %v", err)
			}

			if ok := waitForPollCount(p, tc.ticks, 500*time.Millisecond); !ok {
				p.Stop()
				t.Fatalf("timeout waiting for PollCount >= %d", tc.ticks)
			}

			_, cntBefore := p.Snapshot()
			valBefore := cntBefore[MPollCount]

			p.Stop()
			time.Sleep(3 * tc.interval)

			_, cntAfter := p.Snapshot()
			valAfter := cntAfter[MPollCount]

			if valAfter != valBefore {
				t.Fatalf("PollCount grew after Stop(): before=%d after=%d", valBefore, valAfter)
			}
		})
	}
}
