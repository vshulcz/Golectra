package agent

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/vshulcz/Golectra/internal/config"
)

type Options struct {
	HTTPClient *http.Client
}

type Option func(*Options)

func WithHTTPClient(hc *http.Client) Option {
	return func(o *Options) {
		o.HTTPClient = hc
	}
}

func Run(ctx context.Context, cfg config.AgentConfig) error {
	a, err := New(config.AgentConfig{
		Address:        cfg.Address,
		PollInterval:   cfg.PollInterval,
		ReportInterval: cfg.ReportInterval,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("agent started: server=%s poll=%s report=%s", cfg.Address, cfg.PollInterval, cfg.ReportInterval)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})
	go func() {
		a.Start()
		close(done)
	}()

	select {
	case <-ctx.Done():
		a.Stop()
		<-done
	case <-done:
	}
	return nil
}
