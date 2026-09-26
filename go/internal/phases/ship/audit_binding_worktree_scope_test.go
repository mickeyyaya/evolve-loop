package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// laneAuditedInWorktree seeds a PASS audit for a change staged in a linked worktree, the way a fleet
// lane reaches ship, and returns the plane repo and the worktree.
func laneAuditedInWorktree(t *testing.T) (string, string) {
	t.Helper()
	repo := makeRepo(t)
	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-1", wt)
	mustWrite(t, filepath.Join(wt, "wt-change.txt"), "the lane's audited change\n")
	statePath := filepath.Join(repo, ".evolve", "cycle-state.json")
	state, _ := readStateMap(statePath)
	if state == nil {
		state = map[string]any{}
	}
	state["active_worktree"] = wt
	if err := writeStateMap(statePath, state); err != nil {
		t.Fatal(err)
	}
	seedAudit(t, repo, "PASS")
	return repo, wt
}

// The plane's tracked files hold pipeline bookkeeping (the inbox queue) that operators and sibling
// lanes move while a lane is between audit and ship; none of it is in what this lane ships.
func TestVerifyAuditBinding_PlaneBookkeepingAfterAuditDoesNotUnbindAWorktreeShip(t *testing.T) {
	repo, wt := laneAuditedInWorktree(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "a tracked queue file moved after audit\n")

	opts := auditOpts(t, repo)
	opts.ActiveWorktree = wt
	if err := verifyAuditBinding(context.Background(), opts, &RunResult{}); err != nil {
		t.Fatalf("a plane-side change outside the shipped worktree unbound a passed audit: %v", err)
	}
}

func TestVerifyAuditBinding_APlaneSpelledWithATrailingSlashIsStillThePlane(t *testing.T) {
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS")
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture modified post-audit\n")

	opts := auditOpts(t, repo)
	opts.ActiveWorktree = repo + "/"
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "uncommitted changes")
}

func TestVerifyAuditBinding_AChangeToTheShippedWorktreeAfterAuditStillUnbinds(t *testing.T) {
	repo, wt := laneAuditedInWorktree(t)
	mustWrite(t, filepath.Join(wt, "wt-change.txt"), "edited after audit\n")

	opts := auditOpts(t, repo)
	opts.ActiveWorktree = wt
	err := verifyAuditBinding(context.Background(), opts, &RunResult{})
	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "predicate execution tree-state")
}
