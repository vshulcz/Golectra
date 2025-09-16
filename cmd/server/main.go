package main

import (
	"net/http"
)

func main() {
	run(http.ListenAndServe)
}
