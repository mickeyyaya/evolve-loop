package triagecap

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TriageArtifactName() string {
	if c, ok := phasecontract.For("triage"); ok {
		return c.ArtifactName
	}
	return "triage-report.md"
}

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
