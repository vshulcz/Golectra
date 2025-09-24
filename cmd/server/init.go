package main

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/vshulcz/Golectra/internal/adapters/persistance/file"
	memrepo "github.com/vshulcz/Golectra/internal/adapters/repository/memory"
	pgrepo "github.com/vshulcz/Golectra/internal/adapters/repository/postgres"
	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/ports"
)

func buildRepoAndPersister(cfg config.ServerConfig, logger *zap.Logger) (ports.MetricsRepo, ports.Persister) {
	ctx := context.Background()
	if cfg.DSN != "" {
		db, err := sql.Open("postgres", cfg.DSN)
		if err == nil {
			if err = db.Ping(); err == nil {
				if err = pgrepo.Migrate(db); err == nil {
					logger.Info("db connected & migrated")
					return pgrepo.New(db), nil
				}
			}
		}
		logger.Warn("postgres init failed, falling back to memory", zap.Error(err))
	}
	repo := memrepo.New()
	var p ports.Persister = file.New(cfg.File)
	if cfg.Restore && p != nil {
		if err := p.Restore(ctx, repo); err != nil {
			logger.Warn("restore failed", zap.Error(err))
		} else {
			logger.Info("restore ok", zap.String("file", cfg.File))
		}
	}
	return repo, p
}
