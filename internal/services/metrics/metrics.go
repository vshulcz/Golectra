package metrics

import (
	"context"
	"strings"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Service struct {
	repo      ports.MetricsRepo
	onChanged func(context.Context, domain.Snapshot)
}

func New(repo ports.MetricsRepo, onChanged func(context.Context, domain.Snapshot)) *Service {
	return &Service{repo: repo, onChanged: onChanged}
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *Service) Get(ctx context.Context, mType, id string) (domain.Metrics, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Metrics{}, domain.ErrNotFound
	}
	switch mType {
	case string(domain.Gauge):
		v, err := s.repo.GetGauge(ctx, id)
		if err != nil {
			return domain.Metrics{}, err
		}
		return domain.Metrics{ID: id, MType: mType, Value: &v}, nil
	case string(domain.Counter):
		d, err := s.repo.GetCounter(ctx, id)
		if err != nil {
			return domain.Metrics{}, err
		}
		return domain.Metrics{ID: id, MType: mType, Delta: &d}, nil
	default:
		return domain.Metrics{}, domain.ErrInvalidType
	}
}

func (s *Service) Upsert(ctx context.Context, m domain.Metrics) (domain.Metrics, error) {
	if strings.TrimSpace(m.ID) == "" {
		return domain.Metrics{}, domain.ErrNotFound
	}
	switch m.MType {
	case string(domain.Gauge):
		if m.Value == nil {
			return domain.Metrics{}, domain.ErrInvalidType
		}
		if err := s.repo.SetGauge(ctx, m.ID, *m.Value); err != nil {
			return domain.Metrics{}, err
		}
		return s.Get(ctx, m.MType, m.ID)
	case string(domain.Counter):
		if m.Delta == nil {
			return domain.Metrics{}, domain.ErrInvalidType
		}
		if err := s.repo.AddCounter(ctx, m.ID, *m.Delta); err != nil {
			return domain.Metrics{}, err
		}
		return s.Get(ctx, m.MType, m.ID)
	default:
		return domain.Metrics{}, domain.ErrInvalidType
	}
}

func (s *Service) UpsertBatch(ctx context.Context, items []domain.Metrics) (int, error) {
	valid := make([]domain.Metrics, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.ID) == "" {
			continue
		}
		switch it.MType {
		case string(domain.Gauge):
			if it.Value == nil {
				continue
			}
		case string(domain.Counter):
			if it.Delta == nil {
				continue
			}
		default:
			continue
		}
		valid = append(valid, it)
	}
	if len(valid) == 0 {
		return 0, domain.ErrInvalidType
	}
	if err := s.repo.UpdateMany(ctx, valid); err != nil {
		return 0, err
	}
	if s.onChanged != nil {
		if snap, err := s.repo.Snapshot(ctx); err == nil {
			s.onChanged(ctx, snap)
		}
	}
	return len(valid), nil
}

func (s *Service) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	return s.repo.Snapshot(ctx)
}
