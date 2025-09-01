package agent

import (
	"maps"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

type stats struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func newStats() *stats {
	return &stats{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *stats) setGauge(name string, v float64) {
	s.mu.Lock()
	s.gauges[name] = v
	s.mu.Unlock()
}

func (s *stats) addCounter(name string, d int64) {
	s.mu.Lock()
	s.counters[name] += d
	s.mu.Unlock()
}

func (s *stats) snapshot() (map[string]float64, map[string]int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g := make(map[string]float64, len(s.gauges))
	maps.Copy(g, s.gauges)
	c := make(map[string]int64, len(s.counters))
	maps.Copy(c, s.counters)
	return g, c
}

type poller struct {
	st   *stats
	rnd  *rand.Rand
	stop chan struct{}
}

func newPoller(st *stats) *poller {
	return &poller{
		st:   st,
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())),
		stop: make(chan struct{}),
	}
}

func (p *poller) run(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	var ms runtime.MemStats

	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			runtime.ReadMemStats(&ms)

			p.st.setGauge(MAlloc, float64(ms.Alloc))
			p.st.setGauge(MBuckHashSys, float64(ms.BuckHashSys))
			p.st.setGauge(MFrees, float64(ms.Frees))
			p.st.setGauge(MGCCPUFraction, ms.GCCPUFraction)
			p.st.setGauge(MGCSys, float64(ms.GCSys))
			p.st.setGauge(MHeapAlloc, float64(ms.HeapAlloc))
			p.st.setGauge(MHeapIdle, float64(ms.HeapIdle))
			p.st.setGauge(MHeapInuse, float64(ms.HeapInuse))
			p.st.setGauge(MHeapObjects, float64(ms.HeapObjects))
			p.st.setGauge(MHeapReleased, float64(ms.HeapReleased))
			p.st.setGauge(MHeapSys, float64(ms.HeapSys))
			p.st.setGauge(MLastGC, float64(ms.LastGC))
			p.st.setGauge(MLookups, float64(ms.Lookups))
			p.st.setGauge(MMCacheInuse, float64(ms.MCacheInuse))
			p.st.setGauge(MMCacheSys, float64(ms.MCacheSys))
			p.st.setGauge(MMSpanInuse, float64(ms.MSpanInuse))
			p.st.setGauge(MMSpanSys, float64(ms.MSpanSys))
			p.st.setGauge(MMallocs, float64(ms.Mallocs))
			p.st.setGauge(MNextGC, float64(ms.NextGC))
			p.st.setGauge(MNumForcedGC, float64(ms.NumForcedGC))
			p.st.setGauge(MNumGC, float64(ms.NumGC))
			p.st.setGauge(MOtherSys, float64(ms.OtherSys))
			p.st.setGauge(MPauseTotalNs, float64(ms.PauseTotalNs))
			p.st.setGauge(MStackInuse, float64(ms.StackInuse))
			p.st.setGauge(MStackSys, float64(ms.StackSys))
			p.st.setGauge(MSys, float64(ms.Sys))
			p.st.setGauge(MTotalAlloc, float64(ms.TotalAlloc))

			p.st.setGauge(MRandomValue, p.rnd.Float64())

			p.st.addCounter(MPollCount, 1)
		}
	}
}

func (p *poller) stopNow() {
	close(p.stop)
}
