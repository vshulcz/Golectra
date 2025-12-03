package ports

import (
	"context"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
)

// MetricsCollector periodically gathers gauges and counters from the host/runtime.
type MetricsCollector interface {
	Start(ctx context.Context, interval time.Duration) error
	Stop()
	Snapshot() (gauges map[string]float64, counters map[string]int64)
}

// Publisher delivers metrics to a remote storage endpoint.
type Publisher interface {
	SendBatch(ctx context.Context, items []domain.Metrics) error
	SendOne(ctx context.Context, item domain.Metrics) error
}
