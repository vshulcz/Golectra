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
