package store

import (
	"sync"
	"testing"
)

func TestMemStorage_UpdateAndGetGauge(t *testing.T) {
	ms := NewMemStorage()

	tests := []struct {
		name   string
		key    string
		value  float64
		expect float64
	}{
		{"first set", "Alloc", 123.45, 123.45},
		{"overwrite value", "Alloc", 99.9, 99.9},
		{"new key", "Heap", 42.0, 42.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ms.UpdateGauge(tt.key, tt.value); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := ms.GetGauge(tt.key)
			if !ok {
				t.Fatalf("expected gauge %q to exist", tt.key)
			}
			if got != tt.expect {
				t.Errorf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestMemStorage_UpdateAndGetCounter(t *testing.T) {
	ms := NewMemStorage()

	tests := []struct {
		name     string
		key      string
		deltas   []int64
		expected int64
	}{
		{"single increment", "PollCount", []int64{1}, 1},
		{"multiple increments", "PollCount", []int64{2, 3}, 6},
		{"independent keys", "Another", []int64{10}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, d := range tt.deltas {
				if err := ms.UpdateCounter(tt.key, d); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			got, ok := ms.GetCounter(tt.key)
			if !ok {
				t.Fatalf("expected counter %q to exist", tt.key)
			}
			if got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMemStorage_GetNonexistent(t *testing.T) {
	ms := NewMemStorage()

	if _, ok := ms.GetGauge("missing"); ok {
		t.Error("expected GetGauge to return ok=false for missing key")
	}
	if _, ok := ms.GetCounter("missing"); ok {
		t.Error("expected GetCounter to return ok=false for missing key")
	}
}

func TestMemStorage_ConcurrentAccess(t *testing.T) {
	ms := NewMemStorage()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ms.UpdateGauge("g", float64(i))
			ms.UpdateCounter("c", int64(i))
		}(i)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = ms.GetGauge("g")
			_, _ = ms.GetCounter("c")
		}()
	}
	wg.Wait()
}
