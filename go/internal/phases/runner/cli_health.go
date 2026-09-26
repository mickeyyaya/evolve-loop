package runner

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func runnerLogf(format string, args ...any) { fmt.Fprintf(os.Stderr, "[runner] "+format, args...) }

// applyBenchToPlan reorders the chain by the CLI-health bench unless a policy pin fixes the CLI.
func (b *BaseRunner) applyBenchToPlan(projectRoot, phase string, plan llmroute.Plan, pinned bool, env map[string]string) llmroute.Plan {
	if pinned {
		return plan
	}
	return bridgechain.ApplyCLIHealthBench(projectRoot, phase, plan, env, b.nowFn, runnerLogf)
}

// maybeBenchOnEscalation benches the candidate's family when this dispatch's escalation report classifies a benchable wall.
func (b *BaseRunner) maybeBenchOnEscalation(projectRoot, workspace, candidateCLI string, dispatchStart time.Time, env map[string]string) {
	bridgechain.BenchOnEscalation(projectRoot, workspace, candidateCLI, dispatchStart, env, b.nowFn, runnerLogf)
}
