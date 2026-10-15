package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
)

const crossArtifactInvariantsFile = "crossartifact-invariants.json"

func recordCrossArtifactInvariants(cycle int, workspace, worktree string) {
	if strings.TrimSpace(workspace) == "" {
		return
	}
	report := coherence.CheckCrossArtifactInvariants(workspace, worktree)
	path := filepath.Join(workspace, crossArtifactInvariantsFile)
	buf, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		buf = append(buf, '\n')
		tmp := path + ".tmp"
		if err = os.WriteFile(tmp, buf, 0o644); err == nil {
			err = os.Rename(tmp, path)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN cycle %d could not record %s: %v (advisory only; the cycle is unaffected)\n", cycle, crossArtifactInvariantsFile, err)
		return
	}
	for _, v := range report.Violations() {
		fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d cross-artifact ADVISORY %s: %s (recorded in %s; advisory — this does not block the cycle)\n", cycle, v.Name, v.Evidence, path)
	}
}
