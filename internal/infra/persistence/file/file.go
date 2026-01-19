// Package file implements a filesystem-based metrics snapshot persister.
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

// Persister flushes and restores metric snapshots using a JSON file.
type Persister struct {
	path string
}

// New returns a Persister bound to the provided filesystem path.
func New(path string) *Persister {
	return &Persister{path: path}
}

// Save writes the snapshot to disk atomically.
func (p *Persister) Save(_ context.Context, s domain.Snapshot) error {
	items := flattenSnapshot(s)
	return writeJSONAtomic(p.path, items)
}

// Restore loads metrics from disk and replays them into the provided repository.
func (p *Persister) Restore(ctx context.Context, repo ports.MetricsRepo) (retErr error) {
	f, err := os.Open(p.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close: %w", cerr)
		}
	}()

	var items []domain.Metrics
	if err := json.NewDecoder(f).Decode(&items); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	return repo.UpdateMany(ctx, items)
}

func flattenSnapshot(s domain.Snapshot) []domain.Metrics {
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
	return items
}

func writeJSONAtomic(path string, items []domain.Metrics) (retErr error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
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
		if tmp != nil {
			if cerr := tmp.Close(); cerr != nil && retErr == nil {
				retErr = fmt.Errorf("close tmp: %w", cerr)
			}
		}
		if cleanup {
			if err := os.Remove(tmpName); err != nil && retErr == nil {
				retErr = fmt.Errorf("remove tmp: %w", err)
			}
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
	tmp = nil
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	cleanup = false
	return nil
}
