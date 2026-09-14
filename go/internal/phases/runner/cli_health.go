package runner

// cli_health.go — the runner's two hooks into the CLI-health bench store
// (cycle-283 forensics): consult the bench when building the dispatch chain,
// and write a bench when a dispatch dies on a classified wall. Both are
// disabled by EVOLVE_CLI_HEALTH=0 and bypassed entirely under a policy pin.

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

// runnerLogf is the runner's diagnostic sink for the shared cli-health
// projections: the same stderr lines as before, prefixed "[runner] ".
func runnerLogf(format string, args ...any) { fmt.Fprintf(os.Stderr, "[runner] "+format, args...) }

// applyBenchToPlan is bridgechain.ApplyCLIHealthBench under the runner's
// prefix; bypassed entirely under a policy pin.
func (b *BaseRunner) applyBenchToPlan(projectRoot, phase string, plan llmroute.Plan, pinned bool, env map[string]string) llmroute.Plan {
	if pinned {
		return plan
	}
	return bridgechain.ApplyCLIHealthBench(projectRoot, phase, plan, env, b.nowFn, runnerLogf)
}

// maybeBenchOnEscalation is bridgechain.BenchOnEscalation under the runner's
// prefix: benches candidateCLI's family when the workspace escalation report
// classifies a benchable wall for THIS dispatch.
func (b *BaseRunner) maybeBenchOnEscalation(projectRoot, workspace, candidateCLI string, dispatchStart time.Time, env map[string]string) {
	bridgechain.BenchOnEscalation(projectRoot, workspace, candidateCLI, dispatchStart, env, b.nowFn, runnerLogf)
}
