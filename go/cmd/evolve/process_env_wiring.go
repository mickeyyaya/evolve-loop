package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func init() {
	modelquery.UseProcessEnv(bridge.ProcessEnv)
}
