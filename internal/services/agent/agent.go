package agent

import (
	"context"
	"log"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Service struct {
	collector ports.MetricsCollector
	pub       ports.Publisher
	cfg       config.AgentConfig
}

func New(cfg config.AgentConfig, c ports.MetricsCollector, p ports.Publisher) *Service {
	return &Service{cfg: cfg, collector: c, pub: p}
}

func (r *Service) Run(ctx context.Context) error {
	if err := r.collector.Start(ctx, r.cfg.PollInterval); err != nil {
		return err
	}
	defer r.collector.Stop()

	ticker := time.NewTicker(r.cfg.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.reportOnce(ctx)
		}
	}
}

func (r *Service) reportOnce(ctx context.Context) {
	g, c := r.collector.Snapshot()
	log.Printf("agent: reporting %d gauges, %d counters", len(g), len(c))

	if len(g)+len(c) == 0 {
		return
	}

	batch := make([]domain.Metrics, 0, len(g)+len(c))
	for name, val := range g {
		v := val
		batch = append(batch, domain.Metrics{
			ID:    name,
			MType: string(domain.Gauge),
			Value: &v,
		})
	}
	for name, delta := range c {
		d := delta
		batch = append(batch, domain.Metrics{
			ID:    name,
			MType: string(domain.Counter),
			Delta: &d,
		})
	}

	if err := r.pub.SendBatch(ctx, batch); err != nil {
		log.Printf("agent: batch send failed (%v), fallback to single requests", err)
		for _, m := range batch {
			if err := r.pub.SendOne(ctx, m); err != nil {
				log.Printf("agent: send single failed (%s/%s): %v", m.MType, m.ID, err)
			}
		}
	}
}
