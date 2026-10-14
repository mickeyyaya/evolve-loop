//go:build integration

package ship

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mergedUnpushedFixture(t *testing.T, cycleBranch string) string {
	t.Helper()
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	wt := makeWorktree(t, repo, cycleBranch)
	mustWrite(t, filepath.Join(wt, "feature.txt"), "audited cycle work\n")
	runGit(t, wt, "add", "-A")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")
	boundTree := strings.TrimSpace(runGitOut(t, wt, "rev-parse", "HEAD^{tree}"))

	seedAuditWithBoundTree(t, repo, "PASS", boundTree)

	runGit(t, repo, "merge", "--ff-only", "-q", cycleBranch)

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	return repo
}

func TestRepair_Resume_PostMergePrePush_PushOnlyCloses(t *testing.T) {
	t.Parallel()
	repo := mergedUnpushedFixture(t, "cycle-246-resume")
	preHEAD := headSHA(t, repo)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if res.ExitCode != ExitOK {
		t.Fatalf("resume-unpushed must self-heal push-only; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("RepairAttempted = %q, want AUDIT_BINDING_HEAD_MOVED", res.RepairAttempted)
	}
	if got := headSHA(t, repo); got != preHEAD {
		t.Errorf("HEAD moved during resume (%s → %s) — push-only closure must not commit", preHEAD, got)
	}
	if got := remoteHeadSHA(t, repo); got != preHEAD {
		t.Errorf("remote main = %s, want resumed HEAD %s", got, preHEAD)
	}
	if res.CommitSHA != preHEAD {
		t.Errorf("CommitSHA = %q, want the resumed ship commit %q", res.CommitSHA, preHEAD)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".evolve", "runs", "cycle-1", "ship-binding.json")); statErr != nil {
		t.Errorf("ship-binding.json missing after resume closure: %v", statErr)
	}
}

func TestRepair_Resume_BoundTreeMismatch_NoRepair(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	wt := makeWorktree(t, repo, "cycle-246-mismatch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "work\n")
	runGit(t, wt, "add", "-A")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")

	seedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("d", 40))
	runGit(t, repo, "merge", "--ff-only", "-q", "cycle-246-mismatch")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "should decline"})
	if res.ExitCode == ExitOK {
		t.Fatalf("tree-mismatched HEAD must NOT resume; got ExitOK (logs=%v)", res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Code != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("Code = %s, want AUDIT_BINDING_HEAD_MOVED to stand", se.Code)
	}
	if got := remoteHeadSHA(t, repo); got == headSHA(t, repo) {
		t.Errorf("remote must NOT have been pushed on a declined resume")
	}
}

func TestRepair_Resume_OriginDiverged_NoRepair(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	bare := addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	wt := makeWorktree(t, repo, "cycle-246-diverged")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "work\n")
	runGit(t, wt, "add", "-A")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")
	boundTree := strings.TrimSpace(runGitOut(t, wt, "rev-parse", "HEAD^{tree}"))
	seedAuditWithBoundTree(t, repo, "PASS", boundTree)
	runGit(t, repo, "merge", "--ff-only", "-q", "cycle-246-diverged")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)

	pushDivergentCommit(t, bare)
	divergedRemote := remoteHeadSHA(t, repo)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "should decline"})
	if res.ExitCode == ExitOK {
		t.Fatalf("diverged origin must NOT resume; got ExitOK (logs=%v)", res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Code != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("Code = %s, want AUDIT_BINDING_HEAD_MOVED to stand", se.Code)
	}
	if got := remoteHeadSHA(t, repo); got != divergedRemote {
		t.Errorf("remote main moved (%s → %s) — resume must never force-push", divergedRemote, got)
	}
}

func pushDivergentCommit(t *testing.T, bare string) {
	t.Helper()
	clone := filepath.Join(tempRepoDir(t), "clone2")
	out, err := captureWithEBADFRetry(func() ([]byte, error) {
		cmd := exec.Command("git", "clone", "-q", bare, clone)
		cmd.Env = filteredEnv()
		return cmd.CombinedOutput()
	})
	if err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	runGit(t, clone, "checkout", "-q", "-B", "main", "origin/main")
	runGit(t, clone, "config", "user.email", "second@evolve-loop.test")
	runGit(t, clone, "config", "user.name", "Second Writer")
	mustWrite(t, filepath.Join(clone, "divergent.txt"), "independent change\n")
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "divergent")
	runGit(t, clone, "push", "-q", "origin", "main")
}
