package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_BuildPlanEnvelope pins ADR-0050 Phase 3.7: the build phase's
// upstream build-plan is served via the typed PhaseRequest.BuildPlan envelope at
// advisory+ (populated once at the dispatch seam), with a BYTE-IDENTICAL disk
// fallback at off/shadow (req.BuildPlan empty → the original os.ReadFile path).
func TestComposePrompt_BuildPlanEnvelope(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-plan.md"), []byte("DISK PLAN BODY"), 0o644); err != nil {
		t.Fatal(err)
	}
	plannerOn := map[string]string{"EVOLVE_BUILD_PLANNER": "1"}

	t.Run("envelope build-plan preferred when req.BuildPlan set (advisory+)", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{Workspace: ws, Env: plannerOn, BuildPlan: "ENVELOPE PLAN BODY"})
		if !strings.Contains(got, "## Build Plan\nENVELOPE PLAN BODY") {
			t.Errorf("want envelope plan injected, got:\n%s", got)
		}
		if strings.Contains(got, "DISK PLAN BODY") {
			t.Errorf("disk plan must not be read when the envelope is populated")
		}
	})

	t.Run("disk fallback when envelope empty (off/shadow) — byte-identical", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{Workspace: ws, Env: plannerOn, BuildPlan: ""})
		if !strings.Contains(got, "## Build Plan\nDISK PLAN BODY") {
			t.Errorf("want disk plan injected (unchanged behavior), got:\n%s", got)
		}
	})

	t.Run("planner disabled → no injection regardless of BuildPlan", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{Workspace: ws, Env: map[string]string{}, BuildPlan: "X"})
		if strings.Contains(got, "## Build Plan") {
			t.Errorf("planner off — no build plan should inject, got:\n%s", got)
		}
	})
}
