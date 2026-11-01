//go:build integration

package shiptest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestRepair_Resume_PostMergePrePush_PushOnlyCloses(t *testing.T) {
	t.Parallel()
	repo := shiptest.MergedUnpushedFixture(t, "cycle-246-resume")
	preHEAD := shiptest.HeadSHA(t, repo)

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("resume-unpushed must self-heal push-only; got exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	if res.RepairAttempted != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("RepairAttempted = %q, want AUDIT_BINDING_HEAD_MOVED", res.RepairAttempted)
	}
	if got := shiptest.HeadSHA(t, repo); got != preHEAD {
		t.Errorf("HEAD moved during resume (%s → %s) — push-only closure must not commit", preHEAD, got)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got != preHEAD {
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
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")
	wt := shiptest.MakeWorktree(t, repo, "cycle-246-mismatch")
	shiptest.MustWrite(t, filepath.Join(wt, "feature.txt"), "work\n")
	shiptest.RunGit(t, wt, "add", "-A")
	shiptest.RunGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")

	shiptest.SeedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("d", 40))
	shiptest.RunGit(t, repo, "merge", "--ff-only", "-q", "cycle-246-mismatch")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "should decline"})
	if res.ExitCode == ship.ExitOK {
		t.Fatalf("tree-mismatched HEAD must NOT resume; got ExitOK (logs=%v)", res.Logs)
	}
	se := shiptest.MustShipErr(t, err)
	if se.Code != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("Code = %s, want AUDIT_BINDING_HEAD_MOVED to stand", se.Code)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got == shiptest.HeadSHA(t, repo) {
		t.Errorf("remote must NOT have been pushed on a declined resume")
	}
}

func TestRepair_Resume_OriginDiverged_NoRepair(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	bare := shiptest.AddRemote(t, repo)
	shiptest.RunGit(t, repo, "push", "-q", "origin", "main")
	wt := shiptest.MakeWorktree(t, repo, "cycle-246-diverged")
	shiptest.MustWrite(t, filepath.Join(wt, "feature.txt"), "work\n")
	shiptest.RunGit(t, wt, "add", "-A")
	shiptest.RunGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "cycle work")
	boundTree := strings.TrimSpace(shiptest.RunGitOut(t, wt, "rev-parse", "HEAD^{tree}"))
	shiptest.SeedAuditWithBoundTree(t, repo, "PASS", boundTree)
	shiptest.RunGit(t, repo, "merge", "--ff-only", "-q", "cycle-246-diverged")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"run_id":"test-run","audit_dispatches":1,"phase":"ship","active_worktree":"`+wt+`"}`)

	shiptest.PushDivergentCommit(t, bare)
	divergedRemote := shiptest.RemoteHeadSHA(t, repo)

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "should decline"})
	if res.ExitCode == ship.ExitOK {
		t.Fatalf("diverged origin must NOT resume; got ExitOK (logs=%v)", res.Logs)
	}
	se := shiptest.MustShipErr(t, err)
	if se.Code != "AUDIT_BINDING_HEAD_MOVED" {
		t.Errorf("Code = %s, want AUDIT_BINDING_HEAD_MOVED to stand", se.Code)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got != divergedRemote {
		t.Errorf("remote main moved (%s → %s) — resume must never force-push", divergedRemote, got)
	}
}
