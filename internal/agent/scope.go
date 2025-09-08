package agent

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type runtimeAgent struct {
	cfg    Config
	stats  *stats
	poller *poller
	stop   chan struct{}
}

func NewRuntimeAgent(cfg Config) *runtimeAgent {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.ReportInterval <= 0 {
		cfg.ReportInterval = 10 * time.Second
	}
	if cfg.Address == "" {
		cfg.Address = "http://localhost:8080"
	}
	st := newStats()
	return &runtimeAgent{
		cfg:    cfg,
		stats:  st,
		poller: newPoller(st),
		stop:   make(chan struct{}),
	}
}

func (a *runtimeAgent) Start() {
	go a.poller.run(a.cfg.PollInterval)

	t := time.NewTicker(a.cfg.ReportInterval)
	defer t.Stop()

	for {
		select {
		case <-a.stop:
			a.poller.stopNow()
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
	g, c := a.stats.snapshot()

	log.Printf("agent: reporting %d gauges, %d counters", len(g), len(c))

	for name, val := range g {
		if err := a.postGauge(name, val); err != nil {
			log.Printf("agent: post gauge %s err: %v", name, err)
		}
	}
	for name, val := range c {
		if err := a.postCounter(name, val); err != nil {
			log.Printf("agent: post counter %s err: %v", name, err)
		}
	}
}

func (a *runtimeAgent) postGauge(name string, value float64) error {
	u := fmt.Sprintf("%s/update/gauge/%s/%s",
		a.cfg.Address,
		url.PathEscape(name),
		strconv.FormatFloat(value, 'f', -1, 64),
	)
	return a.post(u)
}

func (a *runtimeAgent) postCounter(name string, delta int64) error {
	u := fmt.Sprintf("%s/update/counter/%s/%s",
		a.cfg.Address,
		url.PathEscape(name),
		strconv.FormatInt(delta, 10),
	)
	return a.post(u)
}

func (a *runtimeAgent) post(urlStr string) error {
	req, err := http.NewRequest(http.MethodPost, urlStr, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server status: %s", resp.Status)
	}
	log.Printf("agent: posted %s -> %s", req.Method, urlStr)
	return nil
}
