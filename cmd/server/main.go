package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	if err := applyServerFlags(os.Args[1:], nil); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	if err := run(http.ListenAndServe); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
