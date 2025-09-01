package main

import (
	"log"
	"net/http"
)

func main() {
	if err := run(http.ListenAndServe); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
