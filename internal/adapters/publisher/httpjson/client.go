package httpjson

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/misc"
	"github.com/vshulcz/Golectra/internal/ports"
)

type Client struct {
	key  string
	base *url.URL
	hc   *http.Client
}

var _ ports.Publisher = (*Client)(nil)

func New(serverAddr string, hc *http.Client, key string) (*Client, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	u, err := url.Parse(normalizeBase(serverAddr))
	if err != nil {
		return nil, err
	}
	return &Client{base: u, hc: hc, key: strings.TrimSpace(key)}, nil
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
	return c.doGzJSON(ctx, "/update", m)
}

func (c *Client) SendBatch(ctx context.Context, metrics []domain.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return c.doGzJSON(ctx, "/updates", metrics)
}

func (c *Client) doGzJSON(ctx context.Context, path string, payload any) error {
	plain, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	var hashHeader string
	if c.key != "" {
		hashHeader = misc.SumSHA256(plain, c.key)
	}

	gz, err := gzipBytes(plain)
	if err != nil {
		return err
	}
	gzBody := gz.Bytes()

	resp, err := c.sendWithRetry(ctx, func() (*http.Request, error) {
		return c.newGzJSONRequest(ctx, path, gzBody, hashHeader)
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := drainAndDiscard(resp); err != nil {
		return err
	}
	return checkHTTPStatus(resp)
}

type httpStatusError struct {
	code int
	msg  string
}

func (e *httpStatusError) Error() string {
	return e.msg
}

func isRetryableHTTP(err error) bool {
	if err == nil {
		return false
	}
	var se *httpStatusError
	if errors.As(err, &se) {
		switch se.code {
		case http.StatusBadGateway, http.StatusServiceUnavailable,
			http.StatusGatewayTimeout, http.StatusTooManyRequests:
			return true
		default:
			return false
		}
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return true
	}
	return errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

func marshalJSON(payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return b, nil
}

func gzipBytes(src []byte) (*bytes.Buffer, error) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(src); err != nil {
		zw.Close()
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return &gz, nil
}

func (c *Client) newGzJSONRequest(ctx context.Context, path string, body []byte, hashHeader string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	if hashHeader != "" {
		req.Header.Set("HashSHA256", hashHeader)
	}

	return req, nil
}

func (c *Client) sendWithRetry(ctx context.Context, mkReq func() (*http.Request, error)) (*http.Response, error) {
	var resp *http.Response
	op := func() error {
		req, err := mkReq()
		if err != nil {
			return err
		}
		r, err := c.hc.Do(req)
		resp = r
		return err
	}
	if err := misc.Retry(ctx, misc.DefaultBackoff, isRetryableHTTP, op); err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	return resp, nil
}

func drainAndDiscard(resp *http.Response) error {
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
	return nil
}

func checkHTTPStatus(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{code: resp.StatusCode, msg: fmt.Sprintf("server status: %s", resp.Status)}
	}
	return nil
}
