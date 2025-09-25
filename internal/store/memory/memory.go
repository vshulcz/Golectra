package memory

import (
	"fmt"
	"maps"
	"sync"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.gauges[name]
	return val, ok
}

func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.counters[name]
	return val, ok
}

func (m *MemStorage) UpdateGauge(name string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
	return nil
}

func (m *MemStorage) UpdateCounter(name string, delta int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += delta
	return nil
}

func (m *MemStorage) Snapshot() (map[string]float64, map[string]int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	g := make(map[string]float64, len(m.gauges))
	maps.Copy(g, m.gauges)
	c := make(map[string]int64, len(m.counters))
	maps.Copy(c, m.counters)
	return g, c
}

func (m *MemStorage) Ping() error {
	return fmt.Errorf("db not configured")
}
