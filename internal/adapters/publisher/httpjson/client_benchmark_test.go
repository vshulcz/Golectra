package httpjson

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vshulcz/Golectra/internal/domain"
)

func BenchmarkClientSendBatch(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			_ = r.Body.Close()
		}()
		var reader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gr, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer func() {
				_ = gr.Close()
			}()
			reader = gr
		}
		_, _ = io.Copy(io.Discard, reader)
		w.WriteHeader(http.StatusOK)
	}))
	b.Cleanup(srv.Close)

	client, err := New(srv.URL, srv.Client(), "bench-secret")
	if err != nil {
		b.Fatalf("new client: %v", err)
	}

	metrics := make([]domain.Metrics, 0, 200)
	for i := range 100 {
		val := float64(i)
		delta := int64(2 * i)
		v := val
		d := delta
		metrics = append(metrics,
			domain.Metrics{ID: fmt.Sprintf("bench-g-%d", i), MType: string(domain.Gauge), Value: &v},
			domain.Metrics{ID: fmt.Sprintf("bench-c-%d", i), MType: string(domain.Counter), Delta: &d},
		)
	}

	ctx := context.Background()
	b.ReportAllocs()

	for b.Loop() {
		if err := client.SendBatch(ctx, metrics); err != nil {
			b.Fatalf("send batch: %v", err)
		}
	}
}
