// gitops_collider_test.go — collider pre-flight false-positive guard.
//
// History: cycle-232 (retro I-10) added the collider PRE-FLIGHT that refuses
// before the worktree commit when untracked main-side files would be
// overwritten by the ff-merge. The repair ladder (ADR-0039 §8,
// operator-approved 2026-06-07) SUPERSEDED the refuse-and-stop contract:
// colliders are now self-healed in-ship — byte-identical copies removed,
// differing copies quarantine-moved — and the merge proceeds. Those behavior
// contracts live in repair_colliders_test.go.
//
// What remains here is the false-positive guard: untracked main-side files
// that do NOT collide with an incoming path must never trip the pre-flight
// (and must never be touched by the repair).
package ship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShipFromWorktree_NoCollider: an untracked main-side file that does
// NOT match any incoming worktree path must not trip the preflight — the
// ship proceeds to ff-merge exactly as before (false-positive guard).
func TestShipFromWorktree_NoCollider(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-232-no-collider")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	// Unrelated untracked main-side file — NOT an incoming path.
	mustWrite(t, filepath.Join(repo, "scratch-unrelated.txt"), "operator scratch\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: no collider"})
	if res.ExitCode != ExitOK {
		t.Fatalf("unrelated untracked file must not block ship; got %d (err=%v, logs=%v)", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "ff-merged cycle-232-no-collider into main") {
		t.Errorf("missing ff-merge log: %v", res.Logs)
	}
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("worktree edit not merged into main; HEAD files: %q", mainFiles)
	}
	// The unrelated untracked file survives, still untracked — the repair
	// must never quarantine or remove a non-colliding operator file.
	got, rerr := os.ReadFile(filepath.Join(repo, "scratch-unrelated.txt"))
	if rerr != nil || string(got) != "operator scratch\n" {
		t.Errorf("unrelated untracked file disturbed (err=%v, content=%q)", rerr, got)
	}
	if res.RepairAttempted != "" {
		t.Errorf("no repair should fire without a collider; RepairAttempted=%q", res.RepairAttempted)
	}
}
