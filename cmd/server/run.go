package main

import (
	"log"
	"net/http"

	"github.com/vshulcz/Golectra/internal/misc"
	"github.com/vshulcz/Golectra/internal/store"
)

func run(listenAndServe func(addr string, handler http.Handler) error) error {
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h)

	addr := misc.Getenv("HTTP_ADDR", "localhost:8080")
	log.Printf("Starting server at http://%s", addr)
	return listenAndServe(addr, r)
}
