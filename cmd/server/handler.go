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
	storage store.Storage
}

func NewHandler(s store.Storage) *Handler {
	return &Handler{storage: s}
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

	case string(models.Counter):
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "bad request")
			return
		}
		h.storage.UpdateCounter(metricName, val)

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
