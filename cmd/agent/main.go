package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vshulcz/Golectra/internal/adapters/collector/runtime"
	"github.com/vshulcz/Golectra/internal/adapters/publisher/httpjson"
	"github.com/vshulcz/Golectra/internal/config"
	agentsvc "github.com/vshulcz/Golectra/internal/services/agent"
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

	pub, err := httpjson.New(cfg.Address, &http.Client{}, cfg.Key)
	if err != nil {
		log.Fatalf("failed to init publisher: %v", err)
	}
	collector := runtime.New()
	runner := agentsvc.New(cfg, collector, pub)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("agent started: server=%s poll=%s report=%s limit=%d",
		cfg.Address, cfg.PollInterval, cfg.ReportInterval, cfg.RateLimit)
	if err := runner.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

func printBuildInfo() {
	util.PrintBuildInfo(buildVersion, buildDate, buildCommit)
}
