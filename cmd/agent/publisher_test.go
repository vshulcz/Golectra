package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vshulcz/Golectra/internal/infra/config"
)

func TestBuildEncrypter_Empty(t *testing.T) {
	cfg := config.AgentConfig{}
	enc, err := buildEncrypter(cfg)
	if err != nil {
		t.Fatalf("buildEncrypter error: %v", err)
	}
	if enc != nil {
		t.Fatal("expected nil encrypter")
	}
}

func TestBuildEncrypter_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(path, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg := config.AgentConfig{CryptoKey: path}
	if _, err := buildEncrypter(cfg); err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestBuildPublisher_GRPC(t *testing.T) {
	cfg := config.AgentConfig{GRPCAddress: "127.0.0.1:1234"}
	pub, cleanup, err := buildPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("buildPublisher error: %v", err)
	}
	if pub == nil || cleanup == nil {
		t.Fatal("expected grpc publisher and cleanup")
	}
	cleanup()
}

func TestBuildPublisher_HTTP(t *testing.T) {
	cfg := config.AgentConfig{Address: "http://localhost:8080"}
	pub, cleanup, err := buildPublisher(cfg, nil)
	if err != nil {
		t.Fatalf("buildPublisher error: %v", err)
	}
	if pub == nil {
		t.Fatal("expected http publisher")
	}
	if cleanup != nil {
		t.Fatal("expected nil cleanup for http publisher")
	}
}

func TestBuildPublisher_HTTPInvalid(t *testing.T) {
	cfg := config.AgentConfig{Address: "http://[::1"}
	if _, _, err := buildPublisher(cfg, nil); err == nil {
		t.Fatal("expected error for invalid address")
	}
}
