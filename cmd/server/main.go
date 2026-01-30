package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/domain"
	auditinfra "github.com/vshulcz/Golectra/internal/infra/audit"
	auditfile "github.com/vshulcz/Golectra/internal/infra/audit/file"
	auditremote "github.com/vshulcz/Golectra/internal/infra/audit/remote"
	"github.com/vshulcz/Golectra/internal/infra/config"
	"github.com/vshulcz/Golectra/internal/infra/crypto/rsaenvelope"
	"github.com/vshulcz/Golectra/internal/infra/http/ginserver"
	"github.com/vshulcz/Golectra/internal/infra/http/ginserver/middlewares"
	"github.com/vshulcz/Golectra/internal/ports"
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

	logger, cleanup, err := initLogger()
	if err != nil {
		return err
	}
	defer cleanup()

	env, err := buildServerEnv(cfg, logger)
	if err != nil {
		return err
	}
	defer env.svc.Close()

	logConfig(cfg)
	ctx, stop := signalContext()
	defer stop()

	return serveWithSignals(ctx, env, logger)
}

func buildAuditor(cfg config.ServerConfig, logger *zap.Logger) ports.AuditPublisher {
	if cfg.AuditFile == "" && cfg.AuditURL == "" {
		return nil
	}
	fanout := auditinfra.NewFanout()
	fanout.SetErrorHandler(func(err error) {
		logger.Warn("audit delivery failed", zap.Error(err))
	})
	if cfg.AuditFile != "" {
		fanout.Attach(auditfile.New(cfg.AuditFile))
	}
	if cfg.AuditURL != "" {
		client, err := auditremote.New(cfg.AuditURL, nil)
		if err != nil {
			logger.Fatal("invalid audit url", zap.Error(err))
		}
		fanout.Attach(client)
	}
	return fanout
}

func initLogger() (*zap.Logger, func(), error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if cerr := logger.Sync(); cerr != nil {
			log.Printf("logger sync: %v", cerr)
		}
	}
	return logger, cleanup, nil
}

func buildSnapshotHook(persister ports.Persister, logger *zap.Logger) func(context.Context, domain.Snapshot) {
	return func(ctx context.Context, s domain.Snapshot) {
		if persister == nil {
			return
		}
		if err := persister.Save(ctx, s); err != nil {
			logger.Warn("save failed", zap.Error(err))
		}
	}
}

func loadDecrypter(cfg config.ServerConfig) (ports.PayloadDecrypter, error) {
	if cfg.CryptoKey == "" {
		return nil, nil
	}
	key, err := rsaenvelope.LoadPrivateKey(cfg.CryptoKey)
	if err != nil {
		return nil, err
	}
	return rsaenvelope.NewDecrypter(key), nil
}

type serverEnv struct {
	repo         ports.MetricsRepo
	persister    ports.Persister
	svc          *metrics.Service
	srv          *http.Server
	stopPeriodic func()
}

func buildServerEnv(cfg config.ServerConfig, logger *zap.Logger) (*serverEnv, error) {
	repo, persister := buildRepoAndPersister(cfg, logger)
	onChanged := buildSnapshotHook(persister, logger)

	auditor := buildAuditor(cfg, logger)
	svc := metrics.New(repo, onChanged, auditor)

	router, err := buildRouter(cfg, logger, svc)
	if err != nil {
		svc.Close()
		return nil, err
	}

	srv := newHTTPServer(cfg, router)
	stopPeriodic := startPeriodicSave(cfg, repo, persister, logger)

	return &serverEnv{
		repo:         repo,
		persister:    persister,
		svc:          svc,
		srv:          srv,
		stopPeriodic: stopPeriodic,
	}, nil
}

func buildRouter(cfg config.ServerConfig, logger *zap.Logger, svc *metrics.Service) (http.Handler, error) {
	h := ginserver.NewHandler(svc)
	decrypter, err := loadDecrypter(cfg)
	if err != nil {
		return nil, err
	}
	subnet, err := parseTrustedSubnet(cfg.TrustedSubnet)
	if err != nil {
		return nil, err
	}

	return ginserver.NewRouter(h, logger,
		middlewares.ZapLogger(logger),
		middlewares.TrustedSubnet(subnet),
		middlewares.DecryptPayload(decrypter),
		middlewares.GzipRequest(),
		middlewares.GzipResponse(),
		middlewares.HashSHA256(cfg.Key),
	), nil
}

func logConfig(cfg config.ServerConfig) {
	log.Printf("cfg: addr=%s file=%s interval=%v restore=%v dsn=%q audit_file=%q audit_url=%q trusted_subnet=%q",
		cfg.Address, cfg.File, cfg.Interval, cfg.Restore, cfg.DSN, cfg.AuditFile, cfg.AuditURL, cfg.TrustedSubnet)
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
}

func parseTrustedSubnet(cidr string) (*net.IPNet, error) {
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return nil, nil
	}
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return subnet, nil
}

func serveWithSignals(ctx context.Context, env *serverEnv, logger *zap.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(env.srv)
	}()

	select {
	case <-ctx.Done():
		env.stopPeriodic()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := env.srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		return saveSnapshot(saveCtx, env.repo, env.persister, logger)
	case err := <-errCh:
		env.stopPeriodic()
		return err
	}
}

func startPeriodicSave(cfg config.ServerConfig, repo ports.MetricsRepo, persister ports.Persister, logger *zap.Logger) func() {
	if cfg.DSN != "" || cfg.Interval <= 0 || persister == nil {
		return func() {}
	}
	ticker := time.NewTicker(cfg.Interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				snap, err := repo.Snapshot(context.Background())
				if err != nil {
					continue
				}
				if err := persister.Save(context.Background(), snap); err != nil {
					logger.Warn("periodic save failed", zap.Error(err))
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()
	return func() {
		close(done)
	}
}

func newHTTPServer(cfg config.ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func serve(srv *http.Server) error {
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func saveSnapshot(ctx context.Context, repo ports.MetricsRepo, persister ports.Persister, logger *zap.Logger) error {
	if repo == nil || persister == nil {
		return nil
	}
	snap, err := repo.Snapshot(ctx)
	if err != nil {
		logger.Warn("snapshot failed", zap.Error(err))
		return err
	}
	if err := persister.Save(ctx, snap); err != nil {
		logger.Warn("snapshot save failed", zap.Error(err))
		return err
	}
	return nil
}

func printBuildInfo() {
	util.PrintBuildInfo(buildVersion, buildDate, buildCommit)
}
