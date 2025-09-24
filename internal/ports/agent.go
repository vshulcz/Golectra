package ports

import (
	"context"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
)

type MetricsCollector interface {
	Start(ctx context.Context, interval time.Duration) error
	Stop()
	Snapshot() (gauges map[string]float64, counters map[string]int64)
}

type Publisher interface {
	SendBatch(ctx context.Context, items []domain.Metrics) error
	SendOne(ctx context.Context, item domain.Metrics) error
}
