package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver"
	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver/middlewares"
	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/metrics"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.LoadServerConfig(os.Args[1:], nil)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	repo, persister := buildRepoAndPersister(cfg, logger)
	onChanged := func(ctx context.Context, s domain.Snapshot) {
		if persister != nil {
			if err := persister.Save(ctx, s); err != nil {
				logger.Warn("save failed", zap.Error(err))
			}
		}
	}

	svc := metrics.New(repo, onChanged)
	h := ginserver.NewHandler(svc)

	r := ginserver.NewRouter(h, logger,
		middlewares.ZapLogger(logger),
		middlewares.GzipRequest(),
		middlewares.GzipResponse(),
		middlewares.HashSHA256(cfg.Key),
	)

	log.Printf("cfg: addr=%s file=%s interval=%v restore=%v dsn=%q",
		cfg.Address, cfg.File, cfg.Interval, cfg.Restore, cfg.DSN)

	if cfg.DSN == "" && cfg.Interval > 0 {
		if cfg.Interval < 0 {
			cfg.Interval = 300 * time.Second
		}
		ticker := time.NewTicker(cfg.Interval)
		go func() {
			for range ticker.C {
				if s, err := repo.Snapshot(context.Background()); err == nil && persister != nil {
					if err := persister.Save(context.Background(), s); err != nil {
						logger.Warn("periodic save failed", zap.Error(err))
					}
				}
			}
		}()
	}

	if err := http.ListenAndServe(cfg.Address, r); err != nil {
		log.Fatal(err)
	}
}
