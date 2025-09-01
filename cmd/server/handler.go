package main

import (
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/vshulcz/Golectra/internal/store"
	"github.com/vshulcz/Golectra/models"
)

var indexTmpl = template.Must(template.New("index").Parse(`
<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<title>metrics</title>
	<style>
		body { font-family: system-ui, Arial, sans-serif; }
		table { border-collapse: collapse; margin-bottom: 1em; }
		td, th { border: 1px solid #ddd; padding: 6px 10px; }
		h2 { margin-top: 1.5em; }
	</style>
</head>
<body>
	<h1>Metrics</h1>

	<h2>Gauge</h2>
	<table>
		<tr><th>Name</th><th>Value</th></tr>
		{{range $name, $val := .Gauge}}
			<tr><td>{{ $name }}</td><td>{{ printf "%.6g" $val }}</td></tr>
		{{else}}
			<tr><td colspan="2"><em>No data</em></td></tr>
		{{end}}
	</table>

	<h2>Counter</h2>
	<table>
		<tr><th>Name</th><th>Value</th></tr>
		{{range $name, $val := .Counter}}
			<tr><td>{{ $name }}</td><td>{{ $val }}</td></tr>
		{{else}}
			<tr><td colspan="2"><em>No data</em></td></tr>
		{{end}}
	</table>
</body>
</html>
`))

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

	data := struct {
		Gauge   map[string]float64
		Counter map[string]int64
	}{
		Gauge:   g,
		Counter: cnt,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := indexTmpl.Execute(c.Writer, data); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}
