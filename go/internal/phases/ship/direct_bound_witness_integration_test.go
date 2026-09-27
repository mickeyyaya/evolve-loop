//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShipDirect_RefusesAnUnsatisfiedBinding(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	seedAudit(t, repo, "PASS")

	// An ordinary uncommitted change in the main tree: no worktree, so
	// atomicShip would route here and shipDirect stages and commits it.
	mustWrite(t, filepath.Join(repo, "direct-change.txt"), "direct-path change\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: direct-path change",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
		// The binding the auditor approved — deliberately not this tree.
		internalAuditBoundTreeSHA: "0000000000000000000000000000000000dead",
	}
	res := &RunResult{}
	err := shipDirect(context.Background(), opts, res, "main")

	if err == nil {
		t.Fatalf("the direct path must reject a commit whose tree does not satisfy its audit binding; it shipped clean and would have written audit_bound_tree_sha into ship-binding.json as if verified — logs: %v", res.Logs)
	}
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeIntegrityTreeDrift {
		t.Fatalf("want CodeIntegrityTreeDrift (the same code the worktree path raises for this class), got %v", err)
	}
	if !strings.Contains(err.Error(), "audit-bound tree SHA") {
		t.Fatalf("the breach must name the binding it failed, got: %v", err)
	}
	if se.Stage != core.StageAtomicShip {
		t.Fatalf("want the PRE-COMMIT guard to fire (stage %q) so the breach is caught before anything lands; got stage %q: %v",
			core.StageAtomicShip, se.Stage, err)
	}
	if !strings.Contains(err.Error(), "staged tree SHA") {
		t.Fatalf("want the pre-commit message shape (staged tree SHA), not the post-push one (committed tree SHA), got: %v", err)
	}
	for _, log := range res.Logs {
		if strings.Contains(log, "committed to") || strings.Contains(log, "pushed to") {
			t.Errorf("the pre-commit guard must refuse BEFORE anything lands, but the run logged %q", log)
		}
	}
}

func TestShipDirect_UnboundShipIsUnaffected(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	mustWrite(t, filepath.Join(repo, "manual-change.txt"), "operator change\n")

	opts := &Options{
		Class:         ClassManual,
		CommitMessage: "chore: operator change",
		ProjectRoot:   repo,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	res := &RunResult{}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("an unbound direct ship must be unaffected by the binding guard: %v", err)
	}
}

func TestShipDirect_LegitimateBoundShipIsNotBlocked(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")

	// The cycle's change must exist BEFORE the audit is sealed, because
	// seedAudit stages the tree and binds exactly what it staged — the same
	// order the live pipeline uses (Builder writes, then Audit binds).
	mustWrite(t, filepath.Join(repo, "direct-change.txt"), "the cycle's change\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "feat: direct-path change",
	})
	if err != nil {
		t.Fatalf("a legitimate bound direct ship must not be blocked, but it was rejected: %v\nlogs: %v", err, res.Logs)
	}
	var verified bool
	for _, log := range res.Logs {
		if strings.Contains(log, "tree-SHA binding verified") {
			verified = true
		}
	}
	if !verified {
		t.Errorf("the ship succeeded but never logged a binding verification — the guard was inert rather than satisfied; logs: %v", res.Logs)
	}
}
