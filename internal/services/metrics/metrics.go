package metrics

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
	"github.com/vshulcz/Golectra/internal/services/audit"
)

// Service exposes business operations for querying and mutating metrics.
type Service struct {
	repo      ports.MetricsRepo
	onChanged func(context.Context, domain.Snapshot)
	auditor   audit.Publisher
	now       func() time.Time
}

// New builds a metrics Service with repository, snapshot hook, and optional auditor.
func New(repo ports.MetricsRepo, onChanged func(context.Context, domain.Snapshot), auditor audit.Publisher) *Service {
	return &Service{repo: repo, onChanged: onChanged, auditor: auditor, now: time.Now}
}

// Ping delegates to the underlying repository health check.
func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

// Get loads a single metric by type and identifier.
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

// Upsert validates and stores one gauge or counter value.
func (s *Service) Upsert(ctx context.Context, m domain.Metrics) (domain.Metrics, error) {
	m.ID = strings.TrimSpace(m.ID)
	if m.ID == "" {
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
		res, err := s.Get(ctx, m.MType, m.ID)
		if err == nil {
			s.notifyAudit(ctx, []string{m.ID})
		}
		return res, err
	case string(domain.Counter):
		if m.Delta == nil {
			return domain.Metrics{}, domain.ErrInvalidType
		}
		if err := s.repo.AddCounter(ctx, m.ID, *m.Delta); err != nil {
			return domain.Metrics{}, err
		}
		res, err := s.Get(ctx, m.MType, m.ID)
		if err == nil {
			s.notifyAudit(ctx, []string{m.ID})
		}
		return res, err
	default:
		return domain.Metrics{}, domain.ErrInvalidType
	}
}

// UpsertBatch applies many metrics in a single repository call and triggers snapshot callbacks.
func (s *Service) UpsertBatch(ctx context.Context, items []domain.Metrics) (int, error) {
	valid := make([]domain.Metrics, 0, len(items))
	names := make([]string, 0, len(items))
	for _, it := range items {
		id := strings.TrimSpace(it.ID)
		if id == "" {
			continue
		}
		it.ID = id
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
		names = append(names, it.ID)
	}
	if len(valid) == 0 {
		return 0, domain.ErrInvalidType
	}
	if err := s.repo.UpdateMany(ctx, valid); err != nil {
		return 0, err
	}
	s.notifyAudit(ctx, names)
	if s.onChanged != nil {
		if snap, err := s.repo.Snapshot(ctx); err == nil {
			s.onChanged(ctx, snap)
		}
	}
	return len(valid), nil
}

// Snapshot returns all known metrics.
func (s *Service) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	return s.repo.Snapshot(ctx)
}

func (s *Service) notifyAudit(ctx context.Context, names []string) {
	if s == nil || s.auditor == nil {
		return
	}
	uniq := dedupNames(names)
	if len(uniq) == 0 {
		return
	}
	var ts int64
	if s.now != nil {
		ts = s.now().Unix()
	}
	evt := audit.Event{
		Timestamp: ts,
		Metrics:   uniq,
		IPAddress: audit.ClientIPFromContext(ctx),
	}
	s.auditor.Publish(ctx, evt)
}

func dedupNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	uniq := names[:0]
	var last string
	for _, name := range names {
		if name == "" || name == last {
			continue
		}
		uniq = append(uniq, name)
		last = name
	}
	return uniq
}
