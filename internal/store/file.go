package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vshulcz/Golectra/models"
)

func SaveToFile(s Storage, path string) error {
	g, c := s.Snapshot()

	items := make([]models.Metrics, 0, len(g)+len(c))
	for k, v := range g {
		vv := v
		items = append(items, models.Metrics{
			ID:    k,
			MType: string(models.Gauge),
			Value: &vv,
		})
	}
	for k, d := range c {
		dd := d
		items = append(items, models.Metrics{
			ID:    k,
			MType: string(models.Counter),
			Delta: &dd,
		})
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		os.MkdirAll(dir, 0o755)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encode: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func LoadFromFile(s Storage, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}
	var items []models.Metrics
	if err := json.Unmarshal(b, &items); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	for _, m := range items {
		switch m.MType {
		case string(models.Gauge):
			if m.Value != nil {
				s.UpdateGauge(m.ID, *m.Value)
			}
		case string(models.Counter):
			if m.Delta != nil {
				s.UpdateCounter(m.ID, *m.Delta)
			}
		}
	}
	return nil
}
