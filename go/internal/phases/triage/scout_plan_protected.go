package triage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func scoutPlanProtectedSection(workspace string) string {
	if workspace == "" {
		return ""
	}
	report, err := os.ReadFile(filepath.Join(workspace, phasecontract.ArtifactFilename(string(core.PhaseScout))))
	if err != nil {
		return ""
	}
	paths := guards.ProtectedPathsIn(string(report))
	if len(paths) == 0 {
		return ""
	}
	return "- scout_plan_protected_paths: scout-report.md names protected control-plane path(s), in a build-plan step or elsewhere: " +
		strings.Join(paths, ", ") + ". A lane can never change them. Check each top_n task against every step of its plan, not only its target files: " +
		"a task whose plan needs one changed is console work; drop it with reason `protected-surface: <path>`, or commit only the part a lane can build.\n"
}
