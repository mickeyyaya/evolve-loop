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

func TestShipFromWorktree_NoCollider(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "cycle-232-no-collider")
	shiptest.MustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	shiptest.MustWrite(t, filepath.Join(repo, "scratch-unrelated.txt"), "operator scratch\n")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: no collider"})
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("unrelated untracked file must not block ship; got %d (err=%v, logs=%v)", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "ff-merged cycle-232-no-collider into main") {
		t.Errorf("missing ff-merge log: %v", res.Logs)
	}
	mainFiles := shiptest.RunGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("worktree edit not merged into main; HEAD files: %q", mainFiles)
	}
	got, rerr := os.ReadFile(filepath.Join(repo, "scratch-unrelated.txt"))
	if rerr != nil || string(got) != "operator scratch\n" {
		t.Errorf("unrelated untracked file disturbed (err=%v, content=%q)", rerr, got)
	}
	if res.RepairAttempted != "" {
		t.Errorf("no repair should fire without a collider; RepairAttempted=%q", res.RepairAttempted)
	}
}
