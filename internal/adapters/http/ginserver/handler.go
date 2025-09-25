package ginserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/domain"
	"github.com/vshulcz/Golectra/internal/services/metrics"
)

type Handler struct {
	svc *metrics.Service
}

func NewHandler(svc *metrics.Service) *Handler {
	return &Handler{svc: svc}
}

// POST /update/:type/:name/:value
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
	if _, err := h.svc.Upsert(c.Request.Context(), m); err != nil {
		httpError(c, err)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("ok"))
}

// GET /value/:type/:name
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

// POST /update  (application/json)
func (h *Handler) UpdateMetricJSON(c *gin.Context) {
	var m domain.Metrics
	if err := c.ShouldBindJSON(&m); err != nil || strings.TrimSpace(m.ID) == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	res, err := h.svc.Upsert(c.Request.Context(), m)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// POST /value  (application/json)
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

// POST /updates (application/json)
func (h *Handler) UpdateMetricsBatchJSON(c *gin.Context) {
	var items []domain.Metrics
	if err := c.ShouldBindJSON(&items); err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	updated, err := h.svc.UpsertBatch(c.Request.Context(), items)
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

// GET /ping
func (h *Handler) Ping(c *gin.Context) {
	if err := h.svc.Ping(c.Request.Context()); err != nil {
		c.String(http.StatusInternalServerError, "db ping error: %v", err)
		return
	}
	c.String(http.StatusOK, "ok")
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
