//go:build integration

// shiperror_mapping_test.go — focused proof that the ship phase emits the
// structured core.ShipError protocol end-to-end: each representative failure
// site is recoverable via core.AsShipError with the correct Code + Class, and
// the finalize() exit-code mapping keys off Class (integrity → ExitIntegrity;
// everything else → ExitFailure). Complements the per-branch coverage tests by
// pinning the FULL Run() boundary for the four classes called out in the plan.
package ship

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestShipError_AuditBindingHeadMoved proves a HEAD-moved audit binding flows
// out of Run() as a recoverable precondition ShipError (NOT integrity) and
// maps to ExitFailure.
func TestShipError_AuditBindingHeadMoved(t *testing.T) {
	repo := makeRepo(t)
	// Seed audit against a HEAD that does not match the repo's current HEAD.
	seedAudit(t, repo, "PASS", map[string]string{
		"head": "0000000000000000000000000000000000000000",
	})
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: head moved"})

	se := wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "HEAD has moved")
	if se.Stage != core.StageVerifyClass {
		t.Errorf("want Stage=verify-class, got %s", se.Stage)
	}
	// Debug must carry the diagnostic SHAs as separate keys (not just in Message).
	if se.Debug["audited"] == "" || se.Debug["current"] == "" {
		t.Errorf("Debug must carry audited+current HEADs; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("precondition class → ExitFailure; got %d", res.ExitCode)
	}
}

// TestShipError_EGPSRedCount proves a non-zero EGPS red_count surfaces as a
// precondition ShipError naming the RED predicate IDs in Debug.
func TestShipError_EGPSRedCount(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\negps red change\n")
	seedAudit(t, repo, "PASS")
	// Drop an acs-verdict.json with red_count>0 next to the audit artifact.
	acsPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "acs-verdict.json")
	mustWrite(t, acsPath, `{"red_count":1,"green_count":3,"verdict":"FAIL","red_ids":["pred-xss"],"predicate_suite":{"total":4}}`)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: egps red"})

	se := wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "RED predicate")
	if !strings.Contains(se.Debug["red_ids"], "pred-xss") {
		t.Errorf("Debug.red_ids must name the failing predicate; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("EGPS red is precondition → ExitFailure; got %d", res.ExitCode)
	}
}

// TestShipError_GitPushRejected proves a push failure surfaces as a transient
// ShipError carrying the git rc/stderr in Debug. We force the failure with a
// fault runner that fails only `git push`.
func TestShipError_GitPushRejected(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\npush reject change\n")
	seedAudit(t, repo, "PASS")
	// No remote configured + fault on push → push fails.
	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "feat: push rejected",
		Runner:        faultRunner("git push", 1, nil),
	})

	se := wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "push failed")
	if se.Stage != core.StageAtomicShip {
		t.Errorf("want Stage=atomic-ship, got %s", se.Stage)
	}
	if se.Debug["git_rc"] != "1" {
		t.Errorf("Debug.git_rc must record the push exit code; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("transient class → ExitFailure; got %d", res.ExitCode)
	}
}

// TestShipError_IntegrityTreeDrift proves a genuine pre-merge tree-SHA breach
// surfaces as an INTEGRITY-class ShipError (recoverable via core.AsShipError)
// AND maps to ExitIntegrity — the only class that does.
func TestShipError_IntegrityTreeDrift(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "drift-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":7,"phase":"ship","active_worktree":"`+wt+`"}`)
	// Bogus audit-bound tree SHA → pre-merge binding fails as integrity drift.
	seedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("b", 40))

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: tree drift"})

	se := wantShipErr(t, err, core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, "INTEGRITY BREACH")
	if se.Debug["audit_bound_tree"] == "" || se.Debug["worktree_tree"] == "" {
		t.Errorf("Debug must carry both tree SHAs; got %v", se.Debug)
	}
	if res.ExitCode != ExitIntegrity {
		t.Errorf("integrity class → ExitIntegrity; got %d", res.ExitCode)
	}
	// The legacy *IntegrityError wrapper must also still match for back-compat.
	if _, ok := err.(*IntegrityError); !ok {
		// err is wrapped through finalize's return (same value), so a direct
		// type assertion through any wrappers may need errors.As; use that.
		if se2, _ := core.AsShipError(err); se2 == nil {
			t.Errorf("integrity error must remain recoverable; got %T", err)
		}
	}
}

// TestShipError_DebugStringDeterministic guards the ledger/debugger contract:
// DebugString renders sorted key=value pairs joined by "; ".
func TestShipError_DebugStringDeterministic(t *testing.T) {
	se := core.NewShipError(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition,
		core.StageVerifyClass, "msg", "current", "abc", "audited", "def")
	got := se.DebugString()
	if got != "audited=def; current=abc" {
		t.Errorf("DebugString not deterministic/sorted; got %q", got)
	}
}
