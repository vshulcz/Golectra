package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/store"
)

func initStorage(cfg config.ServerConfig) store.Storage {
	if cfg.DSN != "" {
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			log.Printf("db open error: %v", err)
		} else {
			if err := db.Ping(); err != nil {
				log.Printf("db ping failed: %v", err)
			} else if err := store.Migrate(db); err != nil {
				log.Printf("db migrate failed: %v", err)
			} else {
				log.Printf("db connected & migrated")
				return store.NewSQLStorage(db)
			}
		}
	}

	base := store.NewMemStorage()
	if cfg.Restore {
		if err := store.LoadFromFile(base, cfg.File); err != nil {
			log.Printf("restore failed: %v", err)
		} else {
			log.Printf("restore ok from %s", cfg.File)
		}
	}

	return base
}

func initPersistence(st store.Storage, h *Handler, cfg config.ServerConfig) {
	if _, ok := st.(*store.SQLStorage); ok {
		return
	}

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
