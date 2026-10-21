package triage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const cycle1788ScoutReport = "## Selected Tasks\n- **Slug:** acs-unsatisfiability-lint\n  - targetFiles: go/internal/evalqualitycheck/\n\n" +
	"## Build Plan\n1. Add the lint to go/internal/evalqualitycheck/unsatisfiable.go.\n" +
	"2. Wire the lint into pre-flight: go/internal/cli/guardcmd/eval.go runs it from `evolve eval quality-check`.\n"

func scoutWorkspace(t *testing.T, report string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestTriageComposePrompt_NamesAProtectedPathThatOnlyABuildPlanStepMentions(t *testing.T) {
	out := hooks{}.ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: scoutWorkspace(t, cycle1788ScoutReport)})

	line := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "- scout_plan_protected_paths:") {
			line = l
		}
	}
	if !strings.Contains(line, "go/internal/cli/guardcmd/eval.go") || !strings.Contains(line, "protected-surface: <path>") || strings.Contains(line, "evalqualitycheck") {
		t.Fatalf("the premise check must name the protected path of plan step 2, and only it (cycle 1788):\n%s", out)
	}
}

func TestTriageComposePrompt_AScoutReportWithNoProtectedPathAddsNoNote(t *testing.T) {
	for name, ws := range map[string]string{
		"lane paths only": scoutWorkspace(t, "## Build Plan\n1. Edit go/internal/evalqualitycheck/unsatisfiable.go.\n"),
		"no scout report": t.TempDir(),
		"no workspace":    "",
	} {
		if out := (hooks{}).ComposePrompt("BODY", core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: ws}); strings.Contains(out, "scout_plan_protected_paths") {
			t.Errorf("%s: the prompt carries a protected-path note with nothing to name:\n%s", name, out)
		}
	}
}
