package triagecap

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TriageArtifactName resolves the triage deliverable filename from the
// contract registry (single source — the same name the runner hook and the
// contract gate use). The registry is a compile-time table, so the lookup
// cannot fail; the fallback literal only guards a future registry refactor.
// Exported for the evalgate floor-binding gate, which reads the same artifact.
func TriageArtifactName() string {
	if c, ok := phasecontract.For("triage"); ok {
		return c.ArtifactName
	}
	return "triage-report.md"
}

// Recorder builds the core.WithThroughputRecorder closure for one project:
// read the shipped cycle's triage artifact from its workspace, count the
// committed floors (committed == passed, since the cycle shipped through the
// gates), and append to the rolling window. Missing artifact or a zero-floor
// cycle is a no-op — only floor-bearing cycles carry throughput signal.
func Recorder(projectRoot string) func(state *core.State, cycle int, workspacePath string) {
	return func(state *core.State, cycle int, workspacePath string) {
		data, err := os.ReadFile(filepath.Join(workspacePath, TriageArtifactName()))
		if err != nil {
			return
		}
		floors := CommittedFloorCount(
			string(data),
			filepath.Join(workspacePath, TriageDecisionName()),
			KnownPackages(projectRoot),
		)
		state.TriageThroughput = Record(state.TriageThroughput, cycle, floors)
	}
}
