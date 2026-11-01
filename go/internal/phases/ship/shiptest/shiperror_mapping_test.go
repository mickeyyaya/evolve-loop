//go:build integration

package shiptest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestShipError_PredicateTreeDrift(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "drift-branch")
	shiptest.MustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":7,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("b", 40))

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: tree drift"})

	se := shiptest.WantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "tree-state mismatch")
	if se.Debug["audited_tree"] == "" || se.Debug["current_tree"] == "" {
		t.Errorf("Debug must carry both tree SHAs; got %v", se.Debug)
	}
	if res.ExitCode != ship.ExitFailure {
		t.Errorf("predicate precondition → ExitFailure; got %d", res.ExitCode)
	}
	if _, ok := err.(*ship.IntegrityError); !ok {
		if se2, _ := core.AsShipError(err); se2 == nil {
			t.Errorf("integrity error must remain recoverable; got %T", err)
		}
	}
}
