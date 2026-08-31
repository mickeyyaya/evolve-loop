package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

// TestBuildExplanationReviewer_RequiresExplanationForMaterialChange is the RED
// contract for the Build -> Audit explanation handoff. A new-schema triage
// decision marks the selected material change as documentation-required; a
// Builder that changes the implementation but omits the explanation
// deliverable must be rejected by the existing correction ladder seam.
func TestBuildExplanationReviewer_RequiresExplanationForMaterialChange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	wt := t.TempDir()
	workspace := filepath.Join(wt, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", wt}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(root, rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	write(wt, "config/app.yaml", "enabled: false\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")

	write(wt, "config/app.yaml", "enabled: true\n")
	git("add", "-A")
	git("commit", "-q", "-m", "builder work")
	write(workspace, "build-report.md", "# Build Report\n\n## Task: enable-app\n\n## Changes\n- `config/app.yaml`\n")
	binding := explanationdocs.CycleBinding{
		ProjectRoot: wt, Worktree: wt, Workspace: workspace, BaseSHA: base,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	if err := explanationdocs.Activate(binding); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	review := NewBuildExplanationReviewer().Review(context.Background(), ReviewInput{
		Phase: string(PhaseBuild), Cycle: 42, RunID: "run-42",
		Workspace: workspace, Worktree: wt, ProjectRoot: wt, WorktreeBaseSHA: base,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if review.Approve || !strings.Contains(review.Reason, "Explanation Documentation") {
		t.Fatalf("material build without explanation documentation must fail the Build floor; got %+v", review)
	}
}

func TestOrchestratorReviewer_ExplanationContractCannotBeReplacedByOptionalReviewer(t *testing.T) {
	in := ReviewInput{
		Phase:                           string(PhaseBuild),
		Cycle:                           42,
		RunID:                           "run-42",
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		ProjectRoot:                     t.TempDir(),
		Worktree:                        t.TempDir(),
		Workspace:                       t.TempDir(),
		WorktreeBaseSHA:                 strings.Repeat("a", 40),
	}
	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{name: "default"},
		{name: "optional reviewer approves", opts: []Option{WithReviewer(noopReviewer{})}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := NewOrchestrator(nil, nil, nil, tc.opts...)
			if got := o.reviewer.Review(context.Background(), in); got.Approve || !strings.Contains(got.Reason, "Explanation Documentation") {
				t.Fatalf("mandatory explanation review was bypassed: %+v", got)
			}
		})
	}
}

func TestMandatoryExplanationReview_FailsClosedWithoutSealedBuildIdentity(t *testing.T) {
	o := NewOrchestrator(nil, nil, nil)
	got := o.reviewer.Review(context.Background(), ReviewInput{
		Phase:                           string(PhaseBuild),
		Cycle:                           42,
		RunID:                           "run-42",
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if got.Approve || !strings.Contains(got.Reason, "Explanation Documentation") {
		t.Fatalf("active contract without worktree/base was approved: %+v", got)
	}
}

func TestSealBuildExplanationContext_FailsClosedWithoutWorktreeOrBase(t *testing.T) {
	err := sealBuildExplanationContext(t.TempDir(), CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: t.TempDir(),
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	})
	if err == nil || !strings.Contains(err.Error(), "worktree and base SHA") {
		t.Fatalf("active contract missing sealed identity err=%v", err)
	}
}
