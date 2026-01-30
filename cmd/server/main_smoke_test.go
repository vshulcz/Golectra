package main

import (
	"testing"

	"github.com/vshulcz/Golectra/internal/application/metrics"
	"github.com/vshulcz/Golectra/internal/infra/config"
)

func TestBuildGRPCServer_InvalidAddress(t *testing.T) {
	svc := metrics.New(&fakeRepo{}, nil, nil)
	defer svc.Close()

	cfg := config.ServerConfig{GRPCAddress: "bad"}
	if _, _, err := buildGRPCServer(cfg, svc); err == nil {
		t.Fatal("expected error for invalid grpc address")
	}
}
