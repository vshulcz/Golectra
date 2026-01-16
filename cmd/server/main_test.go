package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/infra/config"
	"github.com/vshulcz/Golectra/internal/ports"
	"go.uber.org/zap"
)

type fakePersister struct {
	calls int
	err   error
}

func (f *fakePersister) Save(_ context.Context, _ domain.Snapshot) error {
	f.calls++
	return f.err
}

func (f *fakePersister) Restore(context.Context, ports.MetricsRepo) error {
	return nil
}

type fakeRepo struct {
	snapshotCalls int
	snap          domain.Snapshot
	err           error
}

func (r *fakeRepo) GetGauge(context.Context, string) (float64, error)  { return 0, nil }
func (r *fakeRepo) GetCounter(context.Context, string) (int64, error)  { return 0, nil }
func (r *fakeRepo) SetGauge(context.Context, string, float64) error    { return nil }
func (r *fakeRepo) AddCounter(context.Context, string, int64) error    { return nil }
func (r *fakeRepo) UpdateMany(context.Context, []domain.Metrics) error { return nil }
func (r *fakeRepo) Snapshot(context.Context) (domain.Snapshot, error) {
	r.snapshotCalls++
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
	if p.calls != 1 {
		t.Fatalf("calls=%d want 1", p.calls)
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
	if repo.snapshotCalls != 1 {
		t.Fatalf("snapshotCalls=%d want=1", repo.snapshotCalls)
	}
	if p.calls != 1 {
		t.Fatalf("persister calls=%d want=1", p.calls)
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
	for p.calls == 0 {
		select {
		case <-deadline:
			t.Fatal("expected periodic save call")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
