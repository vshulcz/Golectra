package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	agentsvc "github.com/vshulcz/Golectra/internal/application/agent"
	"github.com/vshulcz/Golectra/internal/infra/collector/runtime"
	"github.com/vshulcz/Golectra/internal/infra/config"
	"github.com/vshulcz/Golectra/internal/infra/crypto/rsaenvelope"
	grpcpublisher "github.com/vshulcz/Golectra/internal/infra/publisher/grpc"
	"github.com/vshulcz/Golectra/internal/infra/publisher/httpjson"
	"github.com/vshulcz/Golectra/internal/ports"
	"github.com/vshulcz/Golectra/pkg/util"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	cfg, err := config.LoadAgentConfig(os.Args[1:], nil)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	encrypter, err := buildEncrypter(cfg)
	if err != nil {
		log.Fatalf("failed to load crypto key: %v", err)
	}

	pub, cleanup, err := buildPublisher(cfg, encrypter)
	if err != nil {
		log.Fatalf("failed to init publisher: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	collector := runtime.New()
	runner := agentsvc.New(cfg, collector, pub)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	log.Printf("agent started: server=%s grpc=%s poll=%s report=%s limit=%d",
		cfg.Address, cfg.GRPCAddress, cfg.PollInterval, cfg.ReportInterval, cfg.RateLimit)
	if err := runner.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func buildEncrypter(cfg config.AgentConfig) (ports.PayloadEncrypter, error) {
	if cfg.CryptoKey == "" {
		return nil, nil
	}
	key, err := rsaenvelope.LoadPublicKey(cfg.CryptoKey)
	if err != nil {
		return nil, err
	}
	return rsaenvelope.NewEncrypter(key), nil
}

func buildPublisher(cfg config.AgentConfig, encrypter ports.PayloadEncrypter) (ports.Publisher, func(), error) {
	if cfg.GRPCAddress != "" {
		grpcPub, err := grpcpublisher.New(cfg.GRPCAddress)
		if err != nil {
			return nil, nil, err
		}
		cleanup := func() {
			if err := grpcPub.Close(); err != nil {
				log.Printf("close grpc publisher: %v", err)
			}
		}
		return grpcPub, cleanup, nil
	}
	httpPub, err := httpjson.New(cfg.Address, &http.Client{}, cfg.Key, encrypter)
	if err != nil {
		return nil, nil, err
	}
	return httpPub, nil, nil
}

func printBuildInfo() {
	util.PrintBuildInfo(buildVersion, buildDate, buildCommit)
}
