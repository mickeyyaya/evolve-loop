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

func TestRecoverFromShipError_RebaseInvalidatesExplanationAndReturnsToBuild(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-42")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", worktree}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(worktree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "test")
	git("checkout", "-q", "-b", "main")
	write("base.txt", "base\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	git("checkout", "-q", "-b", "cycle")
	write("lane.txt", "lane change\n")
	git("add", "lane.txt")
	git("commit", "-q", "-m", "lane")
	git("checkout", "-q", "main")
	write("peer.txt", "peer change\n")
	git("add", "peer.txt")
	git("commit", "-q", "-m", "peer")
	newBase := git("rev-parse", "HEAD")
	git("checkout", "-q", "cycle")

	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: worktree, Workspace: workspace, BaseSHA: base,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	doc, err := explanationdocs.DocumentPath(42, "run-42")
	if err != nil {
		t.Fatal(err)
	}
	write(doc, "# Build Explanation\n\n## Build Binding\n- Cycle: 42\n- Base SHA: "+base+"\n\n## Summary\nThe lane behavior changes through one focused source file.\n\n## Rationale\nThe direct implementation is the smallest compatible change and avoids introducing another configuration surface.\n\n## Changed Areas\n- `lane.txt` — contains the lane behavior introduced by this cycle.\n\n## Design Decisions\nThe existing file layout remains the single source of behavior.\n\n## Verification\nThe focused recovery test verifies the complete lifecycle.\n\n## Compatibility\nNo public API changes.\n\n## Limitations\nNo migration is included.\n")
	if err := os.WriteFile(filepath.Join(workspace, "build-report.md"), []byte("## Explanation Documentation\n- Status: REQUIRED\n- Document: "+doc+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}

	storage := &fakeStorage{}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
	cs := CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace, ActiveWorktree: worktree,
		WorktreeBaseSHA: base, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}
	next, recovering := o.recoverFromShipError(context.Background(), root, 42, &cs,
		NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 0, 2)
	if !recovering || next != PhaseBuild {
		t.Fatalf("recovery=(%s,%v), want Build", next, recovering)
	}
	if cs.WorktreeBaseSHA != newBase || storage.cycleState.WorktreeBaseSHA != newBase {
		t.Fatalf("rebased base was not persisted: memory=%q storage=%q want=%q", cs.WorktreeBaseSHA, storage.cycleState.WorktreeBaseSHA, newBase)
	}
	if _, err := explanationdocs.LoadSnapshot(explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: worktree, Workspace: workspace, BaseSHA: newBase,
		Cycle: 42, RunID: "run-42", ContractVersion: explanationdocs.CurrentContractVersion,
	}); err == nil {
		t.Fatal("pre-rebase approved snapshot remained loadable")
	}
}
