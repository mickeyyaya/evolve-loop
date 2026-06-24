package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/directives"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

// makeDirectivesProvider builds the runtime operator-directives provider — the
// SINGLE config boundary for the directives cascade. It is the only place that
// resolves where directives live (home dir) and which loop they target (runscope
// lane); core/orchestrator stay config- and environment-agnostic and just consume
// the returned snapshot each cycle.
//
// Home dir + lane are stable for a loop, so they are resolved once; the directive
// FILES are re-read on every call (directives.Load) so the main session's live
// edits propagate at the next cycle boundary. Fail-open: an empty home or missing
// files yield an empty Set (no directives), never an error or a blocked cycle.
func makeDirectivesProvider(projectRoot string) func(context.Context, int) directives.Set {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fail-open but observable: an unresolved home means the directive files
		// can't be found, so directives are simply off this run (never blocks).
		fmt.Fprintf(os.Stderr, "[directives] WARN: home dir unresolved (%v); operator directives disabled this run\n", err)
	}
	lane := string(runscope.LaneFromRoot(projectRoot))
	globalPath, perLoopPath := directives.Resolve(home, lane)
	return func(_ context.Context, _ int) directives.Set {
		return directives.Load(globalPath, perLoopPath, lane)
	}
}
