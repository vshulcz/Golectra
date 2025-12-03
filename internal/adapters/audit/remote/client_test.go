package remoteaudit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vshulcz/Golectra/internal/services/audit"
)

func TestClient_Notify_OK(t *testing.T) {
	var received audit.Event
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := r.Body.Close(); err != nil {
				t.Fatalf("body close: %v", err)
			}
		}()
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cli, err := New(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	evt := audit.Event{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "1.1.1.1"}
	if err := cli.Notify(context.Background(), evt); err != nil {
		t.Fatalf("Notify error: %v", err)
	}

	if received.IPAddress != evt.IPAddress {
		t.Fatalf("event not forwarded: %+v", received)
	}
}

func TestClient_Notify_StatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	cli, err := New(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	if err := cli.Notify(context.Background(), audit.Event{}); err == nil {
		t.Fatal("expected error")
	}
}
