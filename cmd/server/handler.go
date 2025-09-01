package main

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/vshulcz/Golectra/internal/store"
	"github.com/vshulcz/Golectra/models"
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

	log.Printf("server: recv %s %s CT=%q UA=%q",
		r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.UserAgent())

	path := strings.TrimPrefix(r.URL.Path, "/update/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[1] == "" {
		http.NotFound(w, r)
		return
	}

	metricType, metricName, metricValue := parts[0], parts[1], parts[2]

	switch metricType {
	case string(models.Gauge):
		val, err := strconv.ParseFloat(metricValue, 64)
		if err != nil {
			log.Printf("server: bad gauge %q value %q: %v", metricName, metricValue, err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.storage.UpdateGauge(metricName, val)

	case string(models.Counter):
		val, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			log.Printf("server: bad counter %q value %q: %v", metricName, metricValue, err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		h.storage.UpdateCounter(metricName, val)

	default:
		log.Printf("server: bad metric type %q", metricType)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "ok")
}
