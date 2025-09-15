package main

import (
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/misc"
	"github.com/vshulcz/Golectra/internal/store"
	"go.uber.org/zap"
)

func run(listenAndServe func(addr string, handler http.Handler) error) error {
	st := store.NewMemStorage()
	h := NewHandler(st)

	addr := misc.Getenv("ADDRESS", ":8080")
	file := misc.Getenv("FILE_STORAGE_PATH", "metrics-db.json")
	interval := misc.GetDuration("STORE_INTERVAL", 300*time.Second)
	restore := misc.GetBool("RESTORE", false)

	if restore {
		if err := store.LoadFromFile(st, file); err != nil {
			log.Printf("restore failed: %v", err)
		} else {
			log.Printf("restore ok from %s", file)
		}
	}

	if interval == 0 {
		h.SetAfterUpdate(func() {
			if err := store.SaveToFile(st, file); err != nil {
				log.Printf("save sync failed: %v", err)
			}
		})
	} else {
		if interval < 0 {
			interval = 300 * time.Second
		}
		ticker := time.NewTicker(interval)
		go func() {
			for range ticker.C {
				if err := store.SaveToFile(st, file); err != nil {
					log.Printf("save periodic failed: %v", err)
				} else {
					log.Printf("saved metrics to %s", file)
				}
			}
		}()
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	r := NewRouter(h, logger)

	addr = normalizeURL(addr)
	log.Printf("Starting server at %s (file=%s, interval=%v, restore=%v)", addr, file, interval, restore)
	return listenAndServe(addr, r)
}

func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ":8080"
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Host
		}
	}
	if !strings.Contains(s, ":") {
		return ":" + s
	}
	return s
}
