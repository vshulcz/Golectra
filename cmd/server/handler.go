package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/vshulcz/Golectra/internal/store"
)

type Handler struct {
	storage store.Storage
}

func NewHandler(s store.Storage) *Handler {
	return &Handler{storage: s}
}

func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/update/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] == "" {
		http.NotFound(w, r)
		return
	}

	metricType, metricName, metricValue := parts[0], parts[1], parts[2]

	switch metricType {
	case string(store.Gauge):
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metricName, val)

	case string(store.Counter):
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.storage.UpdateCounter(metricName, val)

	default:
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "ok")
}
