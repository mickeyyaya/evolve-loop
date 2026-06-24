package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_BuildPlanEnvelope pins ADR-0050 Phase 3.7: the build phase's
// upstream build-plan is served exclusively via the typed PhaseRequest.BuildPlan
// envelope (populated once at the dispatch seam when the planner is enabled at
// advisory+). When BuildPlan is empty, no section is injected — the disk fallback
// was removed in cycle-39 alongside the EVOLVE_BUILD_PLANNER env migration.
func TestComposePrompt_BuildPlanEnvelope(t *testing.T) {
	t.Run("envelope build-plan preferred when req.BuildPlan set (advisory+)", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{BuildPlan: "ENVELOPE PLAN BODY"})
		if !strings.Contains(got, "## Build Plan\nENVELOPE PLAN BODY") {
			t.Errorf("want envelope plan injected, got:\n%s", got)
		}
	})

	t.Run("no injection when BuildPlan empty", func(t *testing.T) {
		got := hooks{}.ComposePrompt("body", core.PhaseRequest{BuildPlan: ""})
		if strings.Contains(got, "## Build Plan") {
			t.Errorf("empty BuildPlan must not inject; got:\n%s", got)
		}
	})
}
