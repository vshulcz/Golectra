package agent

import (
	"log"
	"time"

	"github.com/vshulcz/Golectra/internal/agent/collect"
	"github.com/vshulcz/Golectra/internal/agent/state"
	"github.com/vshulcz/Golectra/internal/agent/transport"
	"github.com/vshulcz/Golectra/internal/config"
	"github.com/vshulcz/Golectra/internal/domain"
)

type runtimeAgent struct {
	cfg    config.AgentConfig
	stats  *state.Stats
	poller *collect.Poller
	sender *transport.Client
	stop   chan struct{}
}

func New(cfg config.AgentConfig, opts ...Option) (Agent, error) {
	var o Options
	for _, f := range opts {
		f(&o)
	}

	st := state.New()
	cl, err := transport.NewClient(cfg.Address, o.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &runtimeAgent{
		cfg:    cfg,
		stats:  st,
		poller: collect.New(st),
		sender: cl,
		stop:   make(chan struct{}),
	}, nil
}

func (a *runtimeAgent) Start() {
	go a.poller.Run(a.cfg.PollInterval)

	t := time.NewTicker(a.cfg.ReportInterval)
	defer t.Stop()

	for {
		select {
		case <-a.stop:
			a.poller.Stop()
			return
		case <-t.C:
			a.reportOnce()
		}
	}
}

func (a *runtimeAgent) Stop() {
	close(a.stop)
}

func (a *runtimeAgent) reportOnce() {
	g, c := a.stats.Snapshot()
	log.Printf("agent: reporting %d gauges, %d counters", len(g), len(c))

	for name, val := range g {
		v := val
		a.sender.SendOne(transport.Metric{ID: name, MType: string(domain.Gauge), Value: &v})
	}
	for name, delta := range c {
		d := delta
		a.sender.SendOne(transport.Metric{ID: name, MType: "counter", Delta: &d})
	}
}
