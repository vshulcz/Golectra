package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/infra/config"
	"github.com/vshulcz/Golectra/internal/ports"
	"go.uber.org/zap"
)

type fakePersister struct {
	calls atomic.Int64
	err   error
}

func (f *fakePersister) Save(_ context.Context, _ domain.Snapshot) error {
	f.calls.Add(1)
	return f.err
}

func (f *fakePersister) Restore(context.Context, ports.MetricsRepo) error {
	return nil
}

type fakeRepo struct {
	snapshotCalls atomic.Int64
	snap          domain.Snapshot
	err           error
}

func (r *fakeRepo) GetGauge(context.Context, string) (float64, error)  { return 0, nil }
func (r *fakeRepo) GetCounter(context.Context, string) (int64, error)  { return 0, nil }
func (r *fakeRepo) SetGauge(context.Context, string, float64) error    { return nil }
func (r *fakeRepo) AddCounter(context.Context, string, int64) error    { return nil }
func (r *fakeRepo) UpdateMany(context.Context, []domain.Metrics) error { return nil }
func (r *fakeRepo) Snapshot(context.Context) (domain.Snapshot, error) {
	r.snapshotCalls.Add(1)
	return r.snap, r.err
}
func (r *fakeRepo) Ping(context.Context) error { return nil }

func TestBuildVariablesExist(t *testing.T) {
	_ = buildVersion
	_ = buildDate
	_ = buildCommit
}

func TestBuildSnapshotHook(t *testing.T) {
	logger := zap.NewNop()
	p := &fakePersister{}
	hook := buildSnapshotHook(p, logger)
	hook(context.Background(), domain.Snapshot{})
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("calls=%d want 1", got)
	}
}

func TestLoadDecrypter_Empty(t *testing.T) {
	cfg := config.ServerConfig{}
	if dec, err := loadDecrypter(cfg); err != nil || dec != nil {
		t.Fatalf("loadDecrypter=%v err=%v", dec, err)
	}
}

func TestNewHTTPServer(t *testing.T) {
	cfg := config.ServerConfig{Address: "127.0.0.1:0"}
	srv := newHTTPServer(cfg, http.NewServeMux())
	if srv.Addr != cfg.Address {
		t.Fatalf("Addr=%q want %q", srv.Addr, cfg.Address)
	}
	if srv.ReadTimeout == 0 || srv.WriteTimeout == 0 {
		t.Fatal("timeouts must be set")
	}
}

func TestServe_StartAndClose(t *testing.T) {
	cfg := config.ServerConfig{Address: "127.0.0.1:0"}
	srv := newHTTPServer(cfg, http.NewServeMux())

	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(srv)
	}()

	time.Sleep(10 * time.Millisecond)
	_ = srv.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("serve error: %v", err)
	}
}

func TestSaveSnapshot_NoPersister(t *testing.T) {
	logger := zap.NewNop()
	repo := &fakeRepo{}
	if err := saveSnapshot(context.Background(), repo, nil, logger); err != nil {
		t.Fatalf("saveSnapshot error: %v", err)
	}
}

func TestSaveSnapshot_CallsSave(t *testing.T) {
	logger := zap.NewNop()
	repo := &fakeRepo{snap: domain.Snapshot{}}
	p := &fakePersister{}

	if err := saveSnapshot(context.Background(), repo, p, logger); err != nil {
		t.Fatalf("saveSnapshot error: %v", err)
	}
	if got := repo.snapshotCalls.Load(); got != 1 {
		t.Fatalf("snapshotCalls=%d want=1", got)
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("persister calls=%d want=1", got)
	}
}

func TestBuildRouter(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.ServerConfig{Address: "127.0.0.1:0"}
	svc := metrics.New(&fakeRepo{}, nil, nil)
	defer svc.Close()

	router, err := buildRouter(cfg, logger, svc)
	if err != nil {
		t.Fatalf("buildRouter error: %v", err)
	}
	if router == nil {
		t.Fatal("expected router")
	}
}

func TestBuildAuditor_NoTargets(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.ServerConfig{}
	if got := buildAuditor(cfg, logger); got != nil {
		t.Fatal("expected nil auditor when no targets configured")
	}
}

func TestBuildAuditor_FileTarget(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()
	cfg := config.ServerConfig{AuditFile: dir + "/audit.log"}
	if got := buildAuditor(cfg, logger); got == nil {
		t.Fatal("expected auditor to be configured")
	}
}

func TestInitLogger(t *testing.T) {
	logger, cleanup, err := initLogger()
	if err != nil {
		t.Fatalf("initLogger error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected logger")
	}
	cleanup()
}

func TestParseTrustedSubnet(t *testing.T) {
	subnet, err := parseTrustedSubnet("10.0.0.0/24")
	if err != nil {
		t.Fatalf("parseTrustedSubnet error: %v", err)
	}
	if subnet == nil || subnet.String() != "10.0.0.0/24" {
		t.Fatalf("subnet=%v want 10.0.0.0/24", subnet)
	}

	if _, err := parseTrustedSubnet("bad"); err == nil {
		t.Fatal("expected error for invalid cidr")
	}
	if got, err := parseTrustedSubnet(" "); err != nil || got != nil {
		t.Fatalf("empty subnet: got=%v err=%v", got, err)
	}
}

func TestBuildServerEnv_InvalidSubnet(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.ServerConfig{TrustedSubnet: "bad"}
	if _, err := buildServerEnv(cfg, logger); err == nil {
		t.Fatal("expected error for invalid trusted subnet")
	}
}

func TestBuildServerEnv_Success(t *testing.T) {
	logger := zap.NewNop()
	cfg := config.ServerConfig{
		Address:  "127.0.0.1:0",
		Interval: 0,
	}
	env, err := buildServerEnv(cfg, logger)
	if err != nil {
		t.Fatalf("buildServerEnv error: %v", err)
	}
	env.stopPeriodic()
	env.svc.Close()
}

func TestRun_InvalidArgs(t *testing.T) {
	if err := run([]string{"-a", "http://example.com"}); err == nil {
		t.Fatal("expected error from invalid listen address")
	}
}

func TestServeWithSignals_ShutsDownAndSaves(t *testing.T) {
	logger := zap.NewNop()
	repo := &fakeRepo{snap: domain.Snapshot{}}
	p := &fakePersister{}
	srv := newHTTPServer(config.ServerConfig{Address: "127.0.0.1:0"}, http.NewServeMux())

	stopped := atomic.Bool{}
	env := &serverEnv{
		repo:      repo,
		persister: p,
		svc:       metrics.New(repo, nil, nil),
		srv:       srv,
		stopPeriodic: func() {
			stopped.Store(true)
		},
	}
	defer env.svc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	if err := serveWithSignals(ctx, env, logger); err != nil {
		t.Fatalf("serveWithSignals error: %v", err)
	}
	if !stopped.Load() {
		t.Fatal("expected stopPeriodic to be called")
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("persister calls=%d want=1", got)
	}
}

func TestStartPeriodicSave_TriggersSave(t *testing.T) {
	logger := zap.NewNop()
	repo := &fakeRepo{snap: domain.Snapshot{}}
	p := &fakePersister{}
	cfg := config.ServerConfig{Interval: 10 * time.Millisecond}

	stop := startPeriodicSave(cfg, repo, p, logger)
	defer stop()

	deadline := time.After(200 * time.Millisecond)
	for p.calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("expected periodic save call")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
