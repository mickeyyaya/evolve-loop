package shiptest_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestNativeRun_PostPushExplanationRetryUsesLandedTreeAfterWorktreeCleanup(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.RunGit(t, repo, "branch", "-M", "main")
	worktree := shiptest.MakeWorktree(t, repo, "cycle-42-branch")
	workspace := filepath.Join(repo, ".evolve", "runs", "cycle-42")
	shiptest.MustMkdir(t, workspace)
	base := strings.TrimSpace(shiptest.RunGitOut(t, repo, "rev-parse", "HEAD"))
	binding := explanationdocs.CycleBinding{
		ProjectRoot: repo, Worktree: worktree, Workspace: workspace, BaseSHA: base,
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

	document, err := explanationdocs.DocumentPath(42, "run-42")
	if err != nil {
		t.Fatal(err)
	}
	shiptest.MustWrite(t, filepath.Join(worktree, "fixture.txt"), "fixture line 1\nlanded explanation change\n")
	docBody := "# Build Explanation — Cycle 42\n\n" +
		"## Build Binding\n- Cycle: 42\n- Base SHA: " + base + "\n\n" +
		"## Summary\nThe fixture now records the landed explanation retry behavior.\n\n" +
		"## Rationale\nThe existing tracked fixture gives the retry test a minimal material change with no extra production surface.\n\n" +
		"## Changed Areas\n- `fixture.txt` — adds material content used to verify the landed cycle diff.\n\n" +
		"## Design Decisions\nThe test uses the real linked-worktree lifecycle and one host-owned snapshot.\n\n" +
		"## Verification\nThe native Ship retry must return the existing commit without another mutation.\n\n" +
		"## Compatibility\nNo public interface changes.\n\n" +
		"## Limitations\nCovers the post-push cleanup path only.\n"
	shiptest.MustWrite(t, filepath.Join(worktree, filepath.FromSlash(document)), docBody)
	shiptest.MustWrite(t, filepath.Join(workspace, "build-report.md"), "## Explanation Documentation\n- Status: REQUIRED\n- Document: "+document+"\n")
	if failures := explanationdocs.CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	view, err := explanationdocs.Load(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}

	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":42,"run_id":"run-42","audit_dispatches":1,"active_worktree":"`+worktree+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")
	shiptest.RunGit(t, worktree, "add", "-A")
	shiptest.RunGit(t, worktree, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle 42 landed build")
	shiptest.RunGit(t, repo, "merge", "--ff-only", "cycle-42-branch")
	commit := strings.TrimSpace(shiptest.RunGitOut(t, repo, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(shiptest.RunGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
	shiptest.MustWrite(t, filepath.Join(workspace, "ship-binding.json"), fmt.Sprintf(
		`{"audit_bound_tree_sha":%q,"tree_sha_committed":%q,"commit_sha":%q,"cycle":42}`,
		tree, tree, commit,
	))
	shiptest.RunGit(t, repo, "worktree", "remove", "--force", worktree)
	shiptest.MustWrite(t, filepath.Join(repo, "local-operator-notes.txt"), "unrelated untracked main-side state\n")

	result, err := runShip(t, repo, ship.Options{
		Class: ship.ClassCycle, CommitMessage: "cycle 42 retry", CycleID: 42,
		ActiveWorktree: worktree, WorktreeBaseSHA: base, WorkspacePath: workspace,
		RunID: "run-42", AuditRound: 1, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		BuildExplanation: view, RequireBuildExplanationHandoff: true,
	})
	if err != nil || result.ExitCode != ship.ExitOK || result.CommitSHA != commit || !containsLog(result, "succeeding report-only") {
		t.Fatalf("post-push explanation retry result=%+v err=%v, want report-only commit %s", result, err, commit)
	}
}
