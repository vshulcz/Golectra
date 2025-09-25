package main

import (
	"context"
	"log"
	"os"

	"github.com/vshulcz/Golectra/internal/agent"
	"github.com/vshulcz/Golectra/internal/config"
)

func main() {
	cfg, err := config.LoadAgentConfig(os.Args[1:], nil)
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	if err := agent.Run(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
