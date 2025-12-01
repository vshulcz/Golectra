package ginserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/metrics"
)

type benchRepo struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func newBenchRepo() *benchRepo {
	return &benchRepo{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (r *benchRepo) GetGauge(_ context.Context, name string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.gauges[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *benchRepo) GetCounter(_ context.Context, name string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.counters[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *benchRepo) SetGauge(_ context.Context, name string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[name] = value
	return nil
}

func (r *benchRepo) AddCounter(_ context.Context, name string, delta int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] += delta
	return nil
}

func (r *benchRepo) UpdateMany(ctx context.Context, items []domain.Metrics) error {
	for _, m := range items {
		switch m.MType {
		case string(domain.Gauge):
			if m.Value == nil {
				continue
			}
			if err := r.SetGauge(ctx, m.ID, *m.Value); err != nil {
				return err
			}
		case string(domain.Counter):
			if m.Delta == nil {
				continue
			}
			if err := r.AddCounter(ctx, m.ID, *m.Delta); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *benchRepo) Snapshot(_ context.Context) (domain.Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gauges := make(map[string]float64, len(r.gauges))
	maps.Copy(gauges, r.gauges)
	counters := make(map[string]int64, len(r.counters))
	maps.Copy(counters, r.counters)
	return domain.Snapshot{Gauges: gauges, Counters: counters}, nil
}

func (r *benchRepo) Ping(context.Context) error { return nil }

func BenchmarkHandlerUpdateMetricsBatchJSON(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	repo := newBenchRepo()
	svc := metrics.New(repo, nil, nil)
	handler := NewHandler(svc)

	engine := gin.New()
	engine.POST("/updates", handler.UpdateMetricsBatchJSON)

	items := make([]domain.Metrics, 0, 200)
	for i := range 100 {
		val := float64(i)
		delta := int64(i + 1)
		gVal := val
		cDelta := delta
		items = append(items,
			domain.Metrics{ID: fmt.Sprintf("g-%d", i), MType: string(domain.Gauge), Value: &gVal},
			domain.Metrics{ID: fmt.Sprintf("c-%d", i), MType: string(domain.Counter), Delta: &cDelta},
		)
	}

	payload, err := json.Marshal(items)
	if err != nil {
		b.Fatalf("marshal: %v", err)
	}

	b.ReportAllocs()

	for b.Loop() {
		req := httptest.NewRequest(http.MethodPost, "/updates", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("unexpected status: %d", w.Code)
		}
	}
}
