// gitops_collider_test.go — RED contract for cycle-232 Task 2
// (topology-handles-and-ship-preflight, retro I-10).
//
// shipFromWorktree currently commits in the worktree FIRST and only then
// attempts the ff-merge into main; an untracked main-side file colliding
// with an incoming worktree path makes the merge abort ("would be
// overwritten by merge") AFTER the commit exists, so the orchestrator's
// recovery chain loops audit↔ship (cycle-230 I-10). The fix under test is a
// collider PRE-FLIGHT: before the worktree commit, detect staged worktree
// paths that exist untracked in the main working tree and refuse with a
// precondition-class ShipError that names every collider.
//
// Authored by TDD-Engineer (cycle 232). Builder: make these GREEN without
// modifying this file.
package ship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestShipFromWorktree_ColliderPreflight: an untracked main-side file that
// collides with an incoming worktree path must abort the ship BEFORE the
// worktree commit is created, with a precondition-class ShipError naming
// the collider path. Main must not advance and the untracked main-side
// copy must be preserved byte-for-byte.
func TestShipFromWorktree_ColliderPreflight(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-232-collider-a")
	// Incoming worktree file (Builder's audited edit).
	mustWrite(t, filepath.Join(wt, "collide-a.txt"), "audited worktree content\n")
	// Untracked MAIN-side collider with divergent content (the I-10 scenario).
	mustWrite(t, filepath.Join(repo, "collide-a.txt"), "unaudited main-side copy\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: collider preflight"})
	if res.ExitCode == ExitOK {
		t.Fatalf("ship must refuse on untracked main-side collider; got ExitOK (logs=%v)", res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Class != core.ShipClassPrecondition {
		t.Errorf("want Class=%s, got %s (%v)", core.ShipClassPrecondition, se.Class, err)
	}
	if !strings.Contains(se.Message, "collide-a.txt") {
		t.Errorf("collider error must NAME the colliding path; got %q", se.Message)
	}
	// PRE-flight, not post-mortem: the refusal must happen BEFORE the
	// worktree commit, so the cycle branch is NOT ahead of main afterwards.
	ahead := strings.TrimSpace(runGitOut(t, repo, "rev-list", "--count", "main..cycle-232-collider-a"))
	if ahead != "0" {
		t.Errorf("preflight must refuse BEFORE the worktree commit; branch is %s commit(s) ahead", ahead)
	}
	// Main must not have advanced.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if strings.Contains(mainFiles, "collide-a.txt") {
		t.Errorf("main advanced despite collider; HEAD files: %q", mainFiles)
	}
	// The untracked main-side copy must be preserved byte-for-byte.
	got, rerr := os.ReadFile(filepath.Join(repo, "collide-a.txt"))
	if rerr != nil {
		t.Fatalf("main-side collider file destroyed: %v", rerr)
	}
	if string(got) != "unaudited main-side copy\n" {
		t.Errorf("main-side collider content clobbered; got %q", got)
	}
}

// TestShipFromWorktree_ColliderError_IsActionable: with MULTIPLE colliders
// (including nested paths), the refusal must name EVERY collider so the
// operator can resolve all of them in one pass instead of replaying the
// ship once per file.
func TestShipFromWorktree_ColliderError_IsActionable(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-232-collider-b")
	mustWrite(t, filepath.Join(wt, "acs", "cycle-1", "predicate.sh"), "#!/usr/bin/env bash\nexit 0\n")
	mustWrite(t, filepath.Join(wt, "notes", "topology.md"), "worktree copy\n")
	// Two untracked main-side colliders, one in a nested untracked dir.
	mustWrite(t, filepath.Join(repo, "acs", "cycle-1", "predicate.sh"), "#!/usr/bin/env bash\nexit 1\n")
	mustWrite(t, filepath.Join(repo, "notes", "topology.md"), "stale main copy\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: multi collider"})
	if res.ExitCode == ExitOK {
		t.Fatalf("ship must refuse on colliders; got ExitOK (logs=%v)", res.Logs)
	}
	se := mustShipErr(t, err)
	if se.Class != core.ShipClassPrecondition {
		t.Errorf("want Class=%s, got %s (%v)", core.ShipClassPrecondition, se.Class, err)
	}
	for _, p := range []string{"acs/cycle-1/predicate.sh", "notes/topology.md"} {
		if !strings.Contains(se.Message, p) {
			t.Errorf("actionable collider error must name %q; got %q", p, se.Message)
		}
	}
	// No worktree commit was created.
	ahead := strings.TrimSpace(runGitOut(t, repo, "rev-list", "--count", "main..cycle-232-collider-b"))
	if ahead != "0" {
		t.Errorf("preflight must refuse BEFORE the worktree commit; branch is %s commit(s) ahead", ahead)
	}
}

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
	// The unrelated untracked file survives, still untracked.
	got, rerr := os.ReadFile(filepath.Join(repo, "scratch-unrelated.txt"))
	if rerr != nil || string(got) != "operator scratch\n" {
		t.Errorf("unrelated untracked file disturbed (err=%v, content=%q)", rerr, got)
	}
}
