package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	auditfile "github.com/vshulcz/Golectra/internal/adapters/audit/file"
	auditremote "github.com/vshulcz/Golectra/internal/adapters/audit/remote"
	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver"
	"github.com/vshulcz/Golectra/internal/adapters/http/ginserver/middlewares"
	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/audit"
	"github.com/vshulcz/Golectra/internal/services/metrics"
	"github.com/vshulcz/Golectra/pkg/util"
	"go.uber.org/zap"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := config.LoadServerConfig(args, nil)
	if err != nil {
		return err
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer func() {
		if cerr := logger.Sync(); cerr != nil {
			log.Printf("logger sync: %v", cerr)
		}
	}()

	repo, persister := buildRepoAndPersister(cfg, logger)
	onChanged := func(ctx context.Context, s domain.Snapshot) {
		if persister != nil {
			if err := persister.Save(ctx, s); err != nil {
				logger.Warn("save failed", zap.Error(err))
			}
		}
	}

	auditor := buildAuditor(cfg, logger)
	svc := metrics.New(repo, onChanged, auditor)
	defer svc.Close()
	h := ginserver.NewHandler(svc)

	r := ginserver.NewRouter(h, logger,
		middlewares.ZapLogger(logger),
		middlewares.GzipRequest(),
		middlewares.GzipResponse(),
		middlewares.HashSHA256(cfg.Key),
	)

	log.Printf("cfg: addr=%s file=%s interval=%v restore=%v dsn=%q audit_file=%q audit_url=%q",
		cfg.Address, cfg.File, cfg.Interval, cfg.Restore, cfg.DSN, cfg.AuditFile, cfg.AuditURL)

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

	srv := &http.Server{
		Addr:              cfg.Address,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func buildAuditor(cfg config.ServerConfig, logger *zap.Logger) audit.Publisher {
	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		return nil
	}
	subject := audit.NewSubject()
	subject.SetErrorHandler(func(err error) {
		logger.Warn("audit delivery failed", zap.Error(err))
	})
	if cfg.AuditFile != "" {
		subject.Attach(auditfile.New(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		client, err := auditremote.New(cfg.AuditURL, nil)
		if err != nil {
			logger.Fatal("invalid audit url", zap.Error(err))
		}
		subject.Attach(client)
	}
	return subject
}

func printBuildInfo() {
	util.PrintBuildInfo(buildVersion, buildDate, buildCommit)
}
