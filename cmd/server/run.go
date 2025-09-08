package main

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/vshulcz/Golectra/internal/misc"
	"github.com/vshulcz/Golectra/internal/store"
	"go.uber.org/zap"
)

func run(listenAndServe func(addr string, handler http.Handler) error) error {
	st := store.NewMemStorage()
	h := NewHandler(st)

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	r := NewRouter(h, logger)

	addr := misc.Getenv("ADDRESS", ":8080")
	addr = normalizeURL(addr)
	log.Printf("Starting server at %s", addr)
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
