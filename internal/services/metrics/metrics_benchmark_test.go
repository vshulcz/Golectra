package metrics

import (
	"context"
	"fmt"
	"testing"

	memrepo "github.com/vshulcz/Golectra/internal/adapters/repository/memory"
	"github.com/vshulcz/Golectra/internal/domain"
)

func BenchmarkServiceUpsertBatch(b *testing.B) {
	repo := memrepo.New()
	svc := New(repo, nil, nil)
	ctx := context.Background()

	items := make([]domain.Metrics, 0, 200)
	for i := range 100 {
		val := float64(i)
		delta := int64(i + 1)
		items = append(items,
			domain.Metrics{ID: fmt.Sprintf("g-%d", i), MType: string(domain.Gauge), Value: benchPtrFloat64(val)},
			domain.Metrics{ID: fmt.Sprintf("c-%d", i), MType: string(domain.Counter), Delta: benchPtrInt64(delta)},
		)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := svc.UpsertBatch(ctx, items); err != nil {
			b.Fatalf("UpsertBatch: %v", err)
		}
	}
}

func benchPtrFloat64(v float64) *float64 { vv := v; return &vv }
func benchPtrInt64(v int64) *int64       { vv := v; return &vv }
