package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Persister struct {
	path string
}

func New(path string) *Persister {
	return &Persister{path: path}
}

func (p *Persister) Save(_ context.Context, s domain.Snapshot) error {
	total := len(s.Gauges) + len(s.Counters)
	items := make([]domain.Metrics, 0, total)
	for k, v := range s.Gauges {
		vv := v
		items = append(items, domain.Metrics{ID: k, MType: string(domain.Gauge), Value: &vv})
	}
	for k, d := range s.Counters {
		dd := d
		items = append(items, domain.Metrics{ID: k, MType: string(domain.Counter), Delta: &dd})
	}

	dir := filepath.Dir(p.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}
	tmp, err := os.CreateTemp(dir, ".metrics-*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		tmp.Close()
		if cleanup {
			os.Remove(tmpName)
		}
	}()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpName, p.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	cleanup = false
	return nil
}

func (p *Persister) Restore(ctx context.Context, repo ports.MetricsRepo) error {
	f, err := os.Open(p.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var items []domain.Metrics
	if err := json.NewDecoder(f).Decode(&items); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	return repo.UpdateMany(ctx, items)
}
