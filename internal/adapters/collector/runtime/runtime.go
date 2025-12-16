// Package runtime implements a metrics collector that samples Go runtime stats and host CPU/RAM usage.
package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/vshulcz/Golectra/internal/ports"
)

// Collector periodically samples Go runtime stats plus host CPU/RAM metrics.
type Collector struct {
	st   *stats
	rnd  *rand.Rand
	stop chan struct{}
	wg   sync.WaitGroup
}

var _ ports.MetricsCollector = (*Collector)(nil)

// New creates a Collector with its own gauge storage and random source.
func New() *Collector {
	return &Collector{
		st:   newStats(),
		rnd:  rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404
		stop: make(chan struct{}),
	}
}

// Start launches background goroutines that sample runtime and host metrics at the given interval.
func (c *Collector) Start(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer t.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stop:
				return
			case <-t.C:
				runtime.ReadMemStats(&ms)

				c.st.SetGauge(MAlloc, float64(ms.Alloc))
				c.st.SetGauge(MBuckHashSys, float64(ms.BuckHashSys))
				c.st.SetGauge(MFrees, float64(ms.Frees))
				c.st.SetGauge(MGCCPUFraction, ms.GCCPUFraction)
				c.st.SetGauge(MGCSys, float64(ms.GCSys))
				c.st.SetGauge(MHeapAlloc, float64(ms.HeapAlloc))
				c.st.SetGauge(MHeapIdle, float64(ms.HeapIdle))
				c.st.SetGauge(MHeapInuse, float64(ms.HeapInuse))
				c.st.SetGauge(MHeapObjects, float64(ms.HeapObjects))
				c.st.SetGauge(MHeapReleased, float64(ms.HeapReleased))
				c.st.SetGauge(MHeapSys, float64(ms.HeapSys))
				c.st.SetGauge(MLastGC, float64(ms.LastGC))
				c.st.SetGauge(MLookups, float64(ms.Lookups))
				c.st.SetGauge(MMCacheInuse, float64(ms.MCacheInuse))
				c.st.SetGauge(MMCacheSys, float64(ms.MCacheSys))
				c.st.SetGauge(MMSpanInuse, float64(ms.MSpanInuse))
				c.st.SetGauge(MMSpanSys, float64(ms.MSpanSys))
				c.st.SetGauge(MMallocs, float64(ms.Mallocs))
				c.st.SetGauge(MNextGC, float64(ms.NextGC))
				c.st.SetGauge(MNumForcedGC, float64(ms.NumForcedGC))
				c.st.SetGauge(MNumGC, float64(ms.NumGC))
				c.st.SetGauge(MOtherSys, float64(ms.OtherSys))
				c.st.SetGauge(MPauseTotalNs, float64(ms.PauseTotalNs))
				c.st.SetGauge(MStackInuse, float64(ms.StackInuse))
				c.st.SetGauge(MStackSys, float64(ms.StackSys))
				c.st.SetGauge(MSys, float64(ms.Sys))
				c.st.SetGauge(MTotalAlloc, float64(ms.TotalAlloc))

				c.st.SetGauge(MRandomValue, c.rnd.Float64())
				c.st.AddCounter(MPollCount, 1)
			}
		}
	}()

	tSys := time.NewTicker(interval)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer tSys.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stop:
				return
			case <-tSys.C:
				if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
					c.st.SetGauge(TotalMemory, float64(vm.Total))
					c.st.SetGauge(FreeMemory, float64(vm.Free))
				}
				if pct, err := cpu.Percent(0, true); err == nil {
					for i, p := range pct {
						c.st.SetGauge(fmt.Sprintf("%s%d", CPUutilization, i+1), p)
					}
				}
			}
		}
	}()

	return nil
}

// Stop signals every collector goroutine to halt and waits for them to finish.
func (c *Collector) Stop() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
	c.wg.Wait()
}

// Snapshot returns copies of the latest gauge and counter values.
func (c *Collector) Snapshot() (map[string]float64, map[string]int64) {
	return c.st.Snapshot()
}
