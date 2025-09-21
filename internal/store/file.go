package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vshulcz/Golectra/internal/domain"
)

func SaveToFile(s Storage, path string) error {
	g, c := s.Snapshot()

	total := len(g) + len(c)
	items := make([]domain.Metrics, total)
	i := 0
	for k, v := range g {
		vv := v
		items[i] = domain.Metrics{
			ID:    k,
			MType: string(domain.Gauge),
			Value: &vv,
		}
		i++
	}
	for k, d := range c {
		dd := d
		items[i] = domain.Metrics{
			ID:    k,
			MType: string(domain.Counter),
			Delta: &dd,
		}
		i++
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}

	tmpFile, err := os.CreateTemp(dir, ".metrics-*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmpFile.Name()

	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close tmp: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	cleanup = false
	return nil
}

func LoadFromFile(s Storage, path string) error {
	f, err := os.Open(path)
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

	for _, m := range items {
		switch m.MType {
		case string(domain.Gauge):
			if m.Value != nil {
				s.UpdateGauge(m.ID, *m.Value)
			}
		case string(domain.Counter):
			if m.Delta != nil {
				s.UpdateCounter(m.ID, *m.Delta)
			}
		}
	}
	return nil
}
