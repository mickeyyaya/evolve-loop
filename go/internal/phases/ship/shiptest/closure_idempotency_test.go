//go:build integration

package shiptest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestShip_PostPush_Idempotent_CorrectReportOnly(t *testing.T) {
	t.Parallel()
	repo, _ := shiptest.ShipWorktreeFixture(t, "cycle-1-branch")

	res1, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res1.ExitCode != ship.ExitOK {
		t.Fatalf("fixture ship failed: exit=%d err=%v logs=%v", res1.ExitCode, err, res1.Logs)
	}
	shippedHEAD := shiptest.HeadSHA(t, repo)
	shippedRemote := shiptest.RemoteHeadSHA(t, repo)

	res2, err2 := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})

	if res2.ExitCode != ship.ExitOK {
		t.Fatalf("post-push re-dispatch exit=%d (err=%v) — must be ExitOK report-only, not the cycle-233 AUDIT_BINDING_HEAD_MOVED dead-end; logs=%v",
			res2.ExitCode, err2, res2.Logs)
	}
	if err2 != nil {
		t.Fatalf("post-push re-dispatch returned error: %v", err2)
	}
	if res2.CommitSHA != shippedHEAD {
		t.Errorf("re-dispatch CommitSHA=%q, want the existing ship commit %q", res2.CommitSHA, shippedHEAD)
	}
	if !containsLog(res2, "succeeding report-only") || res2.RepairOutcome == "landing-resumed" {
		t.Errorf("outcome=%q logs=%q: a completed landing intent never resumes; the re-dispatch takes the report-only path", res2.RepairOutcome, res2.Logs)
	}
	if got := shiptest.HeadSHA(t, repo); got != shippedHEAD {
		t.Errorf("HEAD moved on re-dispatch (%s → %s) — post-push correction must not commit", shippedHEAD, got)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got != shippedRemote {
		t.Errorf("remote main moved on re-dispatch (%s → %s) — post-push correction must not push", shippedRemote, got)
	}
}

func TestShip_PrePush_CorrectionStillFullShip(t *testing.T) {
	t.Parallel()
	repo, _ := shiptest.ShipWorktreeFixture(t, "cycle-1b-branch")

	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "ship-binding.json"),
		`{"audit_bound_tree_sha":"","tree_sha_committed":"","commit_sha":"`+strings.Repeat("d", 40)+`","cycle":1}`)

	preHEAD := shiptest.HeadSHA(t, repo)
	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "evolve-cycle 1: goal=test"})
	if err != nil || res.ExitCode != ship.ExitOK {
		t.Fatalf("pre-push ship must run in full: exit=%d err=%v logs=%v", res.ExitCode, err, res.Logs)
	}
	postHEAD := shiptest.HeadSHA(t, repo)
	if postHEAD == preHEAD {
		t.Fatal("HEAD did not advance — the stale binding short-circuited a REAL ship (idempotency fired too eagerly)")
	}
	mainFiles := shiptest.RunGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("feature.txt not in the ship commit; files: %q", mainFiles)
	}
	if got := shiptest.RemoteHeadSHA(t, repo); got != postHEAD {
		t.Errorf("remote main=%s, want pushed HEAD %s", got, postHEAD)
	}
}
