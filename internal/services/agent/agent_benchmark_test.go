package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
)

type benchCollector struct {
	gauges   map[string]float64
	counters map[string]int64
}

func (c *benchCollector) Start(context.Context, time.Duration) error { return nil }
func (c *benchCollector) Stop()                                      {}
func (c *benchCollector) Snapshot() (map[string]float64, map[string]int64) {
	return c.gauges, c.counters
}

type benchPublisher struct{}

func (benchPublisher) SendBatch(context.Context, []domain.Metrics) error { return nil }
func (benchPublisher) SendOne(context.Context, domain.Metrics) error     { return nil }

func BenchmarkAgentReportOnce(b *testing.B) {
	origWriter := log.Writer()
	log.SetOutput(io.Discard)
	b.Cleanup(func() { log.SetOutput(origWriter) })

	gauges := make(map[string]float64, 200)
	counters := make(map[string]int64, 200)
	for i := range 200 {
		gauges[fmt.Sprintf("bench-g-%d", i)] = float64(i)
		counters[fmt.Sprintf("bench-c-%d", i)] = int64(i)
	}

	svc := &Service{
		collector: &benchCollector{gauges: gauges, counters: counters},
		pub:       benchPublisher{},
		cfg: config.AgentConfig{
			RateLimit:      4,
			ReportInterval: time.Second,
			PollInterval:   200 * time.Millisecond,
		},
	}

	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		svc.reportOnce(ctx)
	}
}
