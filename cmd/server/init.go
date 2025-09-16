package main

import (
	"log"
	"time"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/store"
)

func initStorage(cfg config.ServerConfig) *store.MemStorage {
	st := store.NewMemStorage()
	if cfg.Restore {
		if err := store.LoadFromFile(st, cfg.File); err != nil {
			log.Printf("restore failed: %v", err)
		} else {
			log.Printf("restore ok from %s", cfg.File)
		}
	}
	return st
}

func initPersistence(st store.Storage, h *Handler, cfg config.ServerConfig) {
	switch {
	case cfg.Interval == 0:
		h.SetAfterUpdate(func() {
			if err := store.SaveToFile(st, cfg.File); err != nil {
				log.Printf("save sync failed: %v", err)
			}
		})
	case cfg.Interval > 0:
		if cfg.Interval < 0 {
			cfg.Interval = 300 * time.Second
		}
		ticker := time.NewTicker(cfg.Interval)
		go func() {
			for range ticker.C {
				if err := store.SaveToFile(st, cfg.File); err != nil {
					log.Printf("save periodic failed: %v", err)
				}
			}
		}()
	}
}
