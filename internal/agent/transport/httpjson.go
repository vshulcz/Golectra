package transport

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Metric struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

type Client struct {
	base *url.URL
	hc   *http.Client
}

func NewClient(serverAddr string, hc *http.Client) (*Client, error) {
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

func (c *Client) SendOne(metric Metric) error {
	body, err := json.Marshal(metric)
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

	req, err := http.NewRequest(http.MethodPost, c.endpoint("/update"), bytes.NewReader(gz.Bytes()))
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
