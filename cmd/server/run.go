package main

import (
	"log"
	"net/http"

	"github.com/vshulcz/Golectra/internal/store"
)

func run(listenAndServe func(addr string, handler http.Handler) error) error {
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h)

	addr := ":8080"
	log.Printf("Starting server at http://localhost%s\n", addr)
	return listenAndServe(addr, r)
}
