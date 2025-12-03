package ginserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/audit"
	"github.com/vshulcz/Golectra/internal/services/metrics"
)

// Handler exposes HTTP endpoints for metric collection and inspection.
type Handler struct {
	svc *metrics.Service
}

// NewHandler wires a metrics service into a gin-compatible HTTP handler.
func NewHandler(svc *metrics.Service) *Handler {
	return &Handler{svc: svc}
}

var metricsBatchPool = sync.Pool{
	New: func() any {
		batch := make([]domain.Metrics, 0, 256)
		return &batch
	},
}

func decodeMetricsBatch(r io.Reader) ([]domain.Metrics, func(), error) {
	buf := metricsBatchPool.Get().(*[]domain.Metrics)
	items := (*buf)[:0]
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&items); err != nil {
		metricsBatchPool.Put(buf)
		return nil, func() {}, err
	}
	cleanup := func() {
		*buf = items[:0]
		metricsBatchPool.Put(buf)
	}
	return items, cleanup, nil
}

func cloneMetrics(items []domain.Metrics) []domain.Metrics {
	if len(items) == 0 {
		return nil
	}
	clone := make([]domain.Metrics, len(items))
	copy(clone, items)
	return clone
}

// UpdateMetric handles `POST /update/:type/:name/:value` with plain-text payloads.
func (h *Handler) UpdateMetric(c *gin.Context) {
	metricType, metricName, metricValue := c.Param("type"), c.Param("name"), c.Param("value")
	if strings.TrimSpace(metricName) == "" {
		c.String(http.StatusNotFound, "not found")
		return
	}

	var m domain.Metrics
	switch metricType {
	case string(domain.Gauge):
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		m = domain.Metrics{ID: metricName, MType: metricType, Value: &val}

	case string(domain.Counter):
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		m = domain.Metrics{ID: metricName, MType: metricType, Delta: &val}

	default:
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	ctx := audit.WithClientIP(c.Request.Context(), c.ClientIP())
	if _, err := h.svc.Upsert(ctx, m); err != nil {
		httpError(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("ok"))
}

// GetMetric handles `GET /value/:type/:name` returning a plain-text metric value.
func (h *Handler) GetMetric(c *gin.Context) {
	metricType, metricName := c.Param("type"), c.Param("name")

	res, err := h.svc.Get(c.Request.Context(), metricType, metricName)
	if err != nil {
		httpError(c, err)
		return
	}

	switch metricType {
	case string(domain.Gauge):
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strconv.FormatFloat(*res.Value, 'f', -1, 64)))
	case string(domain.Counter):
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strconv.FormatInt(*res.Delta, 10)))
	default:
		c.String(http.StatusBadRequest, "bad request")
	}
}

// Index renders a basic HTML dashboard with current gauge and counter values.
func (h *Handler) Index(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		httpError(c, err)
		return
	}

	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head><meta charset='utf-8'><title>metrics</title>")
	sb.WriteString("<style>body{font-family:system-ui,Arial,sans-serif}table{border-collapse:collapse}td,th{border:1px solid #ddd;padding:6px 10px}</style>")
	sb.WriteString("</head><body>")
	sb.WriteString("<h1>Metrics</h1>")

	sb.WriteString("<h2>Gauge</h2><table><tr><th>Name</th><th>Value</th></tr>")
	for k, v := range snap.Gauges {
		sb.WriteString("<tr><td>")
		sb.WriteString(k)
		sb.WriteString("</td><td>")
		sb.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		sb.WriteString("</td></tr>")
	}
	sb.WriteString("</table>")

	sb.WriteString("<h2>Counter</h2><table><tr><th>Name</th><th>Value</th></tr>")
	for k, v := range snap.Counters {
		sb.WriteString("<tr><td>")
		sb.WriteString(k)
		sb.WriteString("</td><td>")
		sb.WriteString(strconv.FormatInt(v, 10))
		sb.WriteString("</td></tr>")
	}
	sb.WriteString("</table>")

	sb.WriteString("</body></html>")

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(sb.String()))
}

// UpdateMetricJSON handles `POST /update` requests with a JSON metric payload.
func (h *Handler) UpdateMetricJSON(c *gin.Context) {
	var m domain.Metrics
	if err := c.ShouldBindJSON(&m); err != nil || strings.TrimSpace(m.ID) == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	ctx := audit.WithClientIP(c.Request.Context(), c.ClientIP())
	res, err := h.svc.Upsert(ctx, m)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetMetricJSON handles `POST /value` requests returning a metric as JSON.
func (h *Handler) GetMetricJSON(c *gin.Context) {
	var q domain.Metrics
	if err := c.ShouldBindJSON(&q); err != nil || strings.TrimSpace(q.ID) == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	res, err := h.svc.Get(c.Request.Context(), q.MType, q.ID)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// UpdateMetricsBatchJSON handles `POST /updates` with a batch of metrics in JSON.
func (h *Handler) UpdateMetricsBatchJSON(c *gin.Context) {
	items, release, err := decodeMetricsBatch(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	defer release()
	items = cloneMetrics(items)
	ctx := audit.WithClientIP(c.Request.Context(), c.ClientIP())
	updated, err := h.svc.UpsertBatch(ctx, items)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

// Ping proxies `GET /ping` to the storage health check.
func (h *Handler) Ping(c *gin.Context) {
	if err := h.svc.Ping(c.Request.Context()); err != nil {
		c.String(http.StatusInternalServerError, "db ping error: %v", err)
		return
	}
	c.String(http.StatusOK, "ok")
}

// SnapshotJSON handles `GET /api/v1/snapshot` and returns the current snapshot as JSON.
func (h *Handler) SnapshotJSON(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"gauges":   snap.Gauges,
		"counters": snap.Counters,
	})
}

func httpError(c *gin.Context, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, domain.ErrNotFound):
		c.String(http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrInvalidType):
		c.String(http.StatusBadRequest, "bad request")
	default:
		c.String(http.StatusInternalServerError, "internal error")
	}
}
