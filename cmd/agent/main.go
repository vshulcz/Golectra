package main

import (
	"log"
	"os"

	"github.com/vshulcz/Golectra/internal/agent"
)

func main() {
	if err := applyAgentFlags(os.Args[1:], nil); err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	run(func(cfg agent.Config) interface {
		Start()
		Stop()
	} {
		return agent.NewRuntimeAgent(cfg)
	})
}
