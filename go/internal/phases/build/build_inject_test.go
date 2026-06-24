package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestComposePrompt_InjectsBuildPlanWhenPresent verifies the envelope injection
// path: when req.BuildPlan is non-empty (set at the dispatch seam at advisory+
// with planner enabled), ComposePrompt includes the build plan section.
func TestComposePrompt_InjectsBuildPlanWhenPresent(t *testing.T) {
	planContent := "## Implementation Steps\n\n1. Extend ComposePrompt\n"
	req := core.PhaseRequest{
		BuildPlan: planContent,
	}
	result := hooks{}.ComposePrompt("body text", req)
	if !strings.Contains(result, "## Build Plan") {
		t.Errorf("ComposePrompt missing '## Build Plan' section when BuildPlan set; got:\n%s", result)
	}
	if !strings.Contains(result, planContent) {
		t.Errorf("ComposePrompt missing BuildPlan contents; got:\n%s", result)
	}
}

func TestComposePrompt_SkipsBuildPlanWhenAbsent(t *testing.T) {
	req := core.PhaseRequest{} // BuildPlan empty → no injection
	result := hooks{}.ComposePrompt("body text", req)
	if strings.Contains(result, "## Build Plan") {
		t.Errorf("ComposePrompt injected '## Build Plan' when BuildPlan empty; got:\n%s", result)
	}
}
