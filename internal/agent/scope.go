package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/models"
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
	endpoint := mustJoinURL(a.cfg.Address, "/update/")

	log.Printf("agent: reporting %d gauges, %d counters", len(g), len(c))

	for name, val := range g {
		v := val
		msg := models.Metrics{ID: name, MType: string(models.Gauge), Value: &v}
		if err := a.postJSON(endpoint, msg); err != nil {
			log.Printf("agent: post gauge %s err: %v", name, err)
		}
	}
	for name, delta := range c {
		d := delta
		msg := models.Metrics{ID: name, MType: string(models.Counter), Delta: &d}
		if err := a.postJSON(endpoint, msg); err != nil {
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

func (a *runtimeAgent) postJSON(endpoint string, m models.Metrics) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server status: %s", resp.Status)
	}
	return nil
}

func mustJoinURL(base, path string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}
