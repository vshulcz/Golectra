package memory

import (
	"context"
	"errors"
	"maps"
	"sync"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Repo struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

var _ ports.MetricsRepo = (*Repo)(nil)

func New() *Repo {
	return &Repo{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (r *Repo) GetGauge(_ context.Context, name string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.gauges[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *Repo) GetCounter(_ context.Context, name string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.counters[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

func (r *Repo) SetGauge(_ context.Context, name string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[name] = value
	return nil
}

func (r *Repo) AddCounter(_ context.Context, name string, delta int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] += delta
	return nil
}

func (r *Repo) UpdateMany(_ context.Context, items []domain.Metrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
		default:
		}
	}
	return nil
}

func (r *Repo) Snapshot(_ context.Context) (domain.Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g := make(map[string]float64, len(r.gauges))
	maps.Copy(g, r.gauges)
	c := make(map[string]int64, len(r.counters))
	maps.Copy(c, r.counters)
	return domain.Snapshot{Gauges: g, Counters: c}, nil
}

func (*Repo) Ping(context.Context) error {
	return errors.New("db not configured")
}
