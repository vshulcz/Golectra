// Package memory implements an in-memory metrics repository.
package memory

import (
	"context"
	"errors"
	"maps"
	"sync"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

// Repo keeps metrics in memory with coarse-grained RW locking.
type Repo struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

var _ ports.MetricsRepo = (*Repo)(nil)

// New returns an empty in-memory repository.
func New() *Repo {
	return &Repo{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// GetGauge returns the current gauge value or domain.ErrNotFound.
func (r *Repo) GetGauge(_ context.Context, name string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.gauges[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

// GetCounter returns the current counter delta or domain.ErrNotFound.
func (r *Repo) GetCounter(_ context.Context, name string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.counters[name]
	if !ok {
		return 0, domain.ErrNotFound
	}
	return v, nil
}

// SetGauge stores the provided gauge value.
func (r *Repo) SetGauge(_ context.Context, name string, value float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[name] = value
	return nil
}

// AddCounter accumulates the counter delta.
func (r *Repo) AddCounter(_ context.Context, name string, delta int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] += delta
	return nil
}

// UpdateMany applies a batch of gauge/counter updates in-place.
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

// Snapshot copies the current metrics maps to avoid exposing internal state.
func (r *Repo) Snapshot(_ context.Context) (domain.Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g := make(map[string]float64, len(r.gauges))
	maps.Copy(g, r.gauges)
	c := make(map[string]int64, len(r.counters))
	maps.Copy(c, r.counters)
	return domain.Snapshot{Gauges: g, Counters: c}, nil
}

// Ping reports that the in-memory store is not backed by a real database.
func (*Repo) Ping(context.Context) error {
	return errors.New("db not configured")
}
