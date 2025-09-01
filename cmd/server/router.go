package main

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/update/", h.UpdateMetric)
	return mux
}
