package triagecap

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TriageArtifactName is the triage deliverable filename from the phasecontract registry.
func TriageArtifactName() string {
	if c, ok := phasecontract.For("triage"); ok {
		return c.ArtifactName
	}
	return "triage-report.md"
}

// Recorder builds the core.WithThroughputRecorder closure that records a shipped cycle's committed floors.
// A shipped cycle passed its gates, so committed equals passed; a missing artifact or zero floors records nothing.
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
