package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// readUpstreamBuildPlan returns the build phase's upstream build-plan.md body to
// serve via the typed PhaseRequest.BuildPlan envelope (ADR-0050 Phase 3.7). It is
// the dispatch-seam relocation of the ad-hoc os.ReadFile the build phase did
// inside ComposePrompt: same file, read once at the seam so the phase no longer
// reaches to disk for an upstream artifact.
//
// It returns non-empty ONLY at EVOLVE_PHASE_IO>=advisory, for the build phase,
// with the build-planner enabled and a readable file. Every other case returns
// "" so the build phase falls back to its original disk read — byte-identical
// dispatch at off/shadow (the campaign's master no-regression invariant). The
// read is best-effort: a missing/unreadable file yields "" (the phase's own
// fallback then also reads nothing), never an error.
func readUpstreamBuildPlan(stage config.Stage, phase Phase, env map[string]string, workspace string) string {
	if stage < config.StageAdvisory || phase != PhaseBuild || workspace == "" || env["EVOLVE_BUILD_PLANNER"] != "1" {
		return ""
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "build-plan.md")); err == nil {
		return string(data)
	}
	return ""
}
