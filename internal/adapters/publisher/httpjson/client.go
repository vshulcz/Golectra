package httpjson

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Client struct {
	base *url.URL
	hc   *http.Client
}

var _ ports.Publisher = (*Client)(nil)

func New(serverAddr string, hc *http.Client) (*Client, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	u, err := url.Parse(normalizeBase(serverAddr))
	if err != nil {
		return nil, err
	}
	return &Client{base: u, hc: hc}, nil
}

func normalizeBase(s string) string {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return strings.TrimRight(s, "/")
	}
	return "http://" + strings.TrimRight(s, "/")
}

func (c *Client) endpoint(path string) string {
	u := *c.base
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String()
}

func (c *Client) SendOne(ctx context.Context, m domain.Metrics) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(body); err != nil {
		zw.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/update"), bytes.NewReader(gz.Bytes()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("bad gzip: %w", err)
		}
		defer gr.Close()
		r = gr
	}
	io.Copy(io.Discard, r)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server status: %s", resp.Status)
	}
	return nil
}

func (c *Client) SendBatch(ctx context.Context, metrics []domain.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	body, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(body); err != nil {
		zw.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/updates"), bytes.NewReader(gz.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("bad gzip: %w", err)
		}
		defer gr.Close()
		r = gr
	}
	io.Copy(io.Discard, r)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server status: %s", resp.Status)
	}
	return nil
}
