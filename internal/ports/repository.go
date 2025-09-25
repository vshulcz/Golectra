package ports

import (
	"context"

	"github.com/vshulcz/Golectra/internal/domain"
)

type MetricsRepo interface {
	GetGauge(ctx context.Context, name string) (float64, error)
	GetCounter(ctx context.Context, name string) (int64, error)
	SetGauge(ctx context.Context, name string, value float64) error
	AddCounter(ctx context.Context, name string, delta int64) error
	UpdateMany(ctx context.Context, items []domain.Metrics) error

	Snapshot(ctx context.Context) (domain.Snapshot, error)
	Ping(ctx context.Context) error
}

type Persister interface {
	Save(ctx context.Context, s domain.Snapshot) error
	Restore(ctx context.Context, repo MetricsRepo) error
}
