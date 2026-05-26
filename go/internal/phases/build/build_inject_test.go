// These two tests extend go/internal/phases/build/build_test.go.
// Both are RED: current hooks{}.ComposePrompt does not read workspace files.
// Builder must add workspace-file injection to ComposePrompt to make them GREEN.
//
// Copy these test functions verbatim into build_test.go (package build).

package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestComposePrompt_InjectsBuildPlanWhenPresent(t *testing.T) {
	ws := t.TempDir()
	planContent := "## Implementation Steps\n\n1. Extend ComposePrompt\n2. Read build-plan.md from workspace\n"
	if err := os.WriteFile(filepath.Join(ws, "build-plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write build-plan.md: %v", err)
	}
	req := core.PhaseRequest{
		Workspace: ws,
		Env:       map[string]string{"EVOLVE_BUILD_PLANNER": "1"},
	}
	result := hooks{}.ComposePrompt("body text", req)
	if !strings.Contains(result, "## Build Plan") {
		t.Errorf("ComposePrompt missing '## Build Plan' section when file present; got:\n%s", result)
	}
	if !strings.Contains(result, planContent) {
		t.Errorf("ComposePrompt missing build-plan.md contents; got:\n%s", result)
	}
}

func TestComposePrompt_SkipsBuildPlanWhenAbsent(t *testing.T) {
	ws := t.TempDir() // no build-plan.md written
	req := core.PhaseRequest{
		Workspace: ws,
		Env:       map[string]string{"EVOLVE_BUILD_PLANNER": "1"},
	}
	result := hooks{}.ComposePrompt("body text", req)
	if strings.Contains(result, "## Build Plan") {
		t.Errorf("ComposePrompt injected '## Build Plan' when file absent; got:\n%s", result)
	}
}
