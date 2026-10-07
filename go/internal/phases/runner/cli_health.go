package runner

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func runnerLogf(format string, args ...any) { fmt.Fprintf(os.Stderr, "[runner] "+format, args...) }

func (b *BaseRunner) bench(projectRoot, phase string, plan llmroute.Plan, env map[string]string) llmroute.Plan {
	return bridgechain.ApplyCLIHealthBench(projectRoot, phase, plan, env, b.nowFn, runnerLogf)
}

// maybeBenchOnEscalation benches the candidate's family when this dispatch's escalation report classifies a benchable wall.
func (b *BaseRunner) maybeBenchOnEscalation(e bridgechain.Escalation) {
	bridgechain.BenchOnEscalation(e, b.nowFn, runnerLogf)
}
