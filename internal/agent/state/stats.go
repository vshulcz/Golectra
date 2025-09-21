package state

import (
	"maps"
	"sync"
)

type Stats struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func New() *Stats {
	return &Stats{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *Stats) SetGauge(name string, v float64) {
	s.mu.Lock()
	s.gauges[name] = v
	s.mu.Unlock()
}

func (s *Stats) AddCounter(name string, d int64) {
	s.mu.Lock()
	s.counters[name] += d
	s.mu.Unlock()
}

func (s *Stats) Snapshot() (map[string]float64, map[string]int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g := make(map[string]float64, len(s.gauges))
	maps.Copy(g, s.gauges)
	c := make(map[string]int64, len(s.counters))
	maps.Copy(c, s.counters)
	return g, c
}
