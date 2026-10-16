package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

func makeDirectivesProvider(projectRoot string) func(context.Context, int) directives.Set {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[directives] WARN: home dir unresolved (%v); operator directives disabled this run\n", err)
	}
	lane := string(runscope.LaneFromRoot(projectRoot))
	globalPath, perLoopPath := directives.Resolve(home, lane)
	return func(_ context.Context, _ int) directives.Set {
		return directives.Load(globalPath, perLoopPath, lane)
	}
}
