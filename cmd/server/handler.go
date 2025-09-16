package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/store"
	"github.com/vshulcz/Golectra/models"
)

type Handler struct {
	storage     store.Storage
	afterUpdate func()
}

func NewHandler(s store.Storage) *Handler {
	return &Handler{storage: s}
}

func (h *Handler) SetAfterUpdate(fn func()) {
	h.afterUpdate = fn
}

// POST /update/:type/:name/:value
func (h *Handler) UpdateMetric(c *gin.Context) {
	metricType, metricName, metricValue := c.Param("type"), c.Param("name"), c.Param("value")

	if strings.TrimSpace(metricName) == "" {
		c.String(http.StatusNotFound, "not found")
		return
	}

	switch metricType {
	case string(models.Gauge):
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		h.storage.UpdateGauge(metricName, val)
		if h.afterUpdate != nil {
			h.afterUpdate()
		}

	case string(models.Counter):
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		h.storage.UpdateCounter(metricName, val)
		if h.afterUpdate != nil {
			h.afterUpdate()
		}

	default:
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("ok"))
}

// GET /value/:type/:name
func (h *Handler) GetMetric(c *gin.Context) {
	metricType, metricName := c.Param("type"), c.Param("name")

	switch metricType {
	case string(models.Gauge):
		if v, ok := h.storage.GetGauge(metricName); ok {
			c.Data(http.StatusOK, "text/plain; charset=utf-8",
				[]byte(strconv.FormatFloat(v, 'f', -1, 64)))
			return
		}
		c.String(http.StatusNotFound, "not found")
	case string(models.Counter):
		if v, ok := h.storage.GetCounter(metricName); ok {
			c.Data(http.StatusOK, "text/plain; charset=utf-8",
				[]byte(strconv.FormatInt(v, 10)))
			return
		}
		c.String(http.StatusNotFound, "not found")
	default:
		c.String(http.StatusBadRequest, "bad request")
	}
}

func (h *Handler) Index(c *gin.Context) {
	g, cnt := h.storage.Snapshot()

	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head><meta charset='utf-8'><title>metrics</title>")
	sb.WriteString("<style>body{font-family:system-ui,Arial,sans-serif}table{border-collapse:collapse}td,th{border:1px solid #ddd;padding:6px 10px}</style>")
	sb.WriteString("</head><body>")
	sb.WriteString("<h1>Metrics</h1>")

	sb.WriteString("<h2>Gauge</h2><table><tr><th>Name</th><th>Value</th></tr>")
	for k, v := range g {
		sb.WriteString("<tr><td>")
		sb.WriteString(k)
		sb.WriteString("</td><td>")
		sb.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		sb.WriteString("</td></tr>")
	}
	sb.WriteString("</table>")

	sb.WriteString("<h2>Counter</h2><table><tr><th>Name</th><th>Value</th></tr>")
	for k, v := range cnt {
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
	var m models.Metrics
	if err := c.ShouldBindJSON(&m); err != nil || strings.TrimSpace(m.ID) == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	switch m.MType {
	case string(models.Gauge):
		if m.Value == nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		_ = h.storage.UpdateGauge(m.ID, *m.Value)
		if h.afterUpdate != nil {
			h.afterUpdate()
		}
		if v, ok := h.storage.GetGauge(m.ID); ok {
			resp := models.Metrics{ID: m.ID, MType: m.MType, Value: &v}
			c.JSON(http.StatusOK, resp)
			return
		}
		c.String(http.StatusNotFound, "not found")

	case string(models.Counter):
		if m.Delta == nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		_ = h.storage.UpdateCounter(m.ID, *m.Delta)
		if h.afterUpdate != nil {
			h.afterUpdate()
		}
		if v, ok := h.storage.GetCounter(m.ID); ok {
			resp := models.Metrics{ID: m.ID, MType: m.MType, Delta: &v}
			c.JSON(http.StatusOK, resp)
			return
		}
		c.String(http.StatusNotFound, "not found")

	default:
		c.String(http.StatusBadRequest, "bad request")
	}
}

// POST /value  (application/json)
func (h *Handler) GetMetricJSON(c *gin.Context) {
	var q models.Metrics
	if err := c.ShouldBindJSON(&q); err != nil || strings.TrimSpace(q.ID) == "" {
		c.String(http.StatusBadRequest, "bad request")
		return
	}

	switch q.MType {
	case string(models.Gauge):
		if v, ok := h.storage.GetGauge(q.ID); ok {
			resp := models.Metrics{ID: q.ID, MType: q.MType, Value: &v}
			c.JSON(http.StatusOK, resp)
			return
		}
		c.String(http.StatusNotFound, "not found")

	case string(models.Counter):
		if v, ok := h.storage.GetCounter(q.ID); ok {
			resp := models.Metrics{ID: q.ID, MType: q.MType, Delta: &v}
			c.JSON(http.StatusOK, resp)
			return
		}
		c.String(http.StatusNotFound, "not found")

	default:
		c.String(http.StatusBadRequest, "bad request")
	}
}
