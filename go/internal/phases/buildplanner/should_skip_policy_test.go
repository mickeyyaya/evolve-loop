package buildplanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func projectWithMandatory(t *testing.T, phase string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"mandatory_phases":["`+phase+`"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestShouldSkip_APolicyMandatoryPhaseRunsAndADegradedDigestWarns(t *testing.T) {
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "handoff-triage.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	skip, _, _, diags := New(Config{}).ShouldSkip(core.PhaseRequest{Workspace: ws, ProjectRoot: projectWithMandatory(t, "build-planner")})
	if skip {
		t.Error("a build-planner in the policy mandatory_phases must run")
	}
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "routing digest degraded") {
		t.Errorf("diags = %+v, want one degraded-digest warning", diags)
	}
}

func TestShouldSkip_AMissingWorkspaceWarnsThatTheDigestFailed(t *testing.T) {
	_, _, _, diags := New(Config{}).ShouldSkip(core.PhaseRequest{Workspace: filepath.Join(t.TempDir(), "absent"), ProjectRoot: t.TempDir()})
	if len(diags) == 0 || !strings.Contains(diags[0].Message, "routing digest failed") {
		t.Errorf("diags = %+v, want a digest-failed warning", diags)
	}
}
