package remoteaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vshulcz/Golectra/internal/services/audit"
)

// Client sends audit events to a remote HTTP endpoint.
type Client struct {
	endpoint string
	hc       *http.Client
}

// New validates the endpoint URL and returns a Client that POSTs audit events there.
func New(rawURL string, hc *http.Client) (*Client, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("audit url is empty")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return nil, fmt.Errorf("invalid audit url: %w", err)
	}
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{endpoint: rawURL, hc: hc}, nil
}

// Notify serializes the audit event and issues an HTTP POST to the configured endpoint.
func (c *Client) Notify(ctx context.Context, evt audit.Event) (retErr error) {
	if c == nil {
		return nil
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("audit post: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("close audit response: %w", cerr)
		}
	}()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("drain audit response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audit post status %d", resp.StatusCode)
	}
	return nil
}
