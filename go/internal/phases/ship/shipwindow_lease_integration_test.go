//go:build integration

package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_TheRepoContractPackRunsOutsideTheShipWindow(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-pack")
	mustWrite(t, filepath.Join(wt, "go", "go.mod"), evolveLoopGoMod)
	mustWrite(t, filepath.Join(wt, "go", "internal", "lane", "lane.go"), "package lane\n\nconst Lane = 1\n")
	runGit(t, wt, "add", "-A")
	boundTree := strings.TrimSpace(runGitOut(t, wt, "write-tree"))
	base := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAuditWithBoundTree(t, repo, "PASS", boundTree)
	var packRuns []bool
	prev := repoContractTestFn
	t.Cleanup(func() { repoContractTestFn = prev })
	repoContractTestFn = func(ctx context.Context, moduleDir string, out packLog) packOutcome {
		packRuns = append(packRuns, leaseHeld(repo))
		return greenPack()
	}
	var pushes []bool
	observe := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && len(args) > 0 && args[0] == "push" {
			pushes = append(pushes, leaseHeld(repo))
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}

	p := New(Config{Runner: observe, RepoContractGate: "enforce"})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: repo, Workspace: filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		Worktree: wt, WorktreeBaseSHA: base, Context: map[string]string{"commit_message": "evolve-cycle 1: goal=test"},
	})

	if err != nil || resp.Verdict != core.VerdictPASS {
		t.Fatalf("precondition: the lane ships, got verdict=%s err=%v diagnostics=%v", resp.Verdict, err, resp.Diagnostics)
	}
	if len(packRuns) == 0 || len(pushes) == 0 {
		t.Fatalf("precondition: the pack ran %d time(s) and ship pushed %d time(s)", len(packRuns), len(pushes))
	}
	for i, held := range packRuns {
		if held {
			t.Errorf("repo-contract pack run %d held the ship window, so a sibling waits out the whole scan", i)
		}
	}
	for i, held := range pushes {
		if !held {
			t.Errorf("push %d ran outside the ship window", i)
		}
	}
}
