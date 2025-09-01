package main

import (
	"log"
	"net/http"

	"github.com/vshulcz/Golectra/internal/store"
)

func main() {
	st := store.NewMemStorage()
	h := NewHandler(st)
	r := NewRouter(h)

	addr := ":8080"
	log.Printf("Starting server at http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
