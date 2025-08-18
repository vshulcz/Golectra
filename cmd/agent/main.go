package main

import (
	"github.com/vshulcz/Golectra/internal/agent"
)

func main() {
	run(func(cfg agent.Config) interface {
		Start()
		Stop()
	} {
		return agent.NewRuntimeAgent(cfg)
	})
}
