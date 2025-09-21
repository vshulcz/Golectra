package collect

import (
	"math/rand"
	"runtime"
	"time"

	"github.com/vshulcz/Golectra/internal/agent/state"
)

type Poller struct {
	st   *state.Stats
	rnd  *rand.Rand
	stop chan struct{}
}

func New(st *state.Stats) *Poller {
	return &Poller{
		st:   st,
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())),
		stop: make(chan struct{}),
	}
}

func (p *Poller) Run(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	var ms runtime.MemStats

	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			runtime.ReadMemStats(&ms)

			p.st.SetGauge(MAlloc, float64(ms.Alloc))
			p.st.SetGauge(MBuckHashSys, float64(ms.BuckHashSys))
			p.st.SetGauge(MFrees, float64(ms.Frees))
			p.st.SetGauge(MGCCPUFraction, ms.GCCPUFraction)
			p.st.SetGauge(MGCSys, float64(ms.GCSys))
			p.st.SetGauge(MHeapAlloc, float64(ms.HeapAlloc))
			p.st.SetGauge(MHeapIdle, float64(ms.HeapIdle))
			p.st.SetGauge(MHeapInuse, float64(ms.HeapInuse))
			p.st.SetGauge(MHeapObjects, float64(ms.HeapObjects))
			p.st.SetGauge(MHeapReleased, float64(ms.HeapReleased))
			p.st.SetGauge(MHeapSys, float64(ms.HeapSys))
			p.st.SetGauge(MLastGC, float64(ms.LastGC))
			p.st.SetGauge(MLookups, float64(ms.Lookups))
			p.st.SetGauge(MMCacheInuse, float64(ms.MCacheInuse))
			p.st.SetGauge(MMCacheSys, float64(ms.MCacheSys))
			p.st.SetGauge(MMSpanInuse, float64(ms.MSpanInuse))
			p.st.SetGauge(MMSpanSys, float64(ms.MSpanSys))
			p.st.SetGauge(MMallocs, float64(ms.Mallocs))
			p.st.SetGauge(MNextGC, float64(ms.NextGC))
			p.st.SetGauge(MNumForcedGC, float64(ms.NumForcedGC))
			p.st.SetGauge(MNumGC, float64(ms.NumGC))
			p.st.SetGauge(MOtherSys, float64(ms.OtherSys))
			p.st.SetGauge(MPauseTotalNs, float64(ms.PauseTotalNs))
			p.st.SetGauge(MStackInuse, float64(ms.StackInuse))
			p.st.SetGauge(MStackSys, float64(ms.StackSys))
			p.st.SetGauge(MSys, float64(ms.Sys))
			p.st.SetGauge(MTotalAlloc, float64(ms.TotalAlloc))

			p.st.SetGauge(MRandomValue, p.rnd.Float64())

			p.st.AddCounter(MPollCount, 1)
		}
	}
}

func (p *Poller) Stop() {
	close(p.stop)
}
