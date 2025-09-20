package main

import (
	"log"
	"net/http"
	"os"

	"github.com/vshulcz/Golectra/internal/config"
	"go.uber.org/zap"
)

func run(listenAndServe func(addr string, handler http.Handler) error) error {
	cfg, err := config.LoadServerConfig(os.Args[1:], nil)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	st := initStorage(cfg)
	h := NewHandler(st)
	initPersistence(st, h, cfg)

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	r := NewRouter(h, logger)

	log.Printf("cfg: addr=%s file=%s interval=%v restore=%v dsn=%q",
		cfg.Address, cfg.File, cfg.Interval, cfg.Restore, cfg.DSN)
	return listenAndServe(cfg.Address, r)
}
