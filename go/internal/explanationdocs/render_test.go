package explanationdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderNotApplicableDeclaration_RoundTripsThroughTheValidator(t *testing.T) {
	report := "# Build Report\n\n## Files Modified\n- none\n\n" + RenderNotApplicableDeclaration("the walk produces no Build diff")
	decl, present, err := parseDeclaration(report)
	if err != nil || !present {
		t.Fatalf("parseDeclaration: present=%v err=%v", present, err)
	}
	if decl.Status != "NOT_APPLICABLE" || decl.Reason != "the walk produces no Build diff" || decl.Document != "" {
		t.Fatalf("declaration = %+v", decl)
	}
	root := t.TempDir()
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	binding := CycleBinding{ProjectRoot: root, Worktree: root, Workspace: workspace, Cycle: 1, RunID: "run-1", ContractVersion: 1}
	if failures, fault := checkNotApplicable(binding, decl, nil, ""); len(failures) != 0 || fault != nil {
		t.Fatalf("the validator rejects its own render: %v %v", failures, fault)
	}
	if !strings.HasSuffix(report, "\n") {
		t.Error("the render ends its last bullet with a newline so a following section parses")
	}
}
