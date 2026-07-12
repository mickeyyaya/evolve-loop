//go:build acs

// Package cycle752 materializes the cycle-752 acceptance criteria for the sole
// committed top_n task inbox-promotion-requires-landed-ship (triage-report.md
// ## top_n; scout's three selections were out of this fleet lane's assigned
// set and dropped, so per R9.3 no predicates bind to them and no
// deferred-floor predicates exist).
//
// Task source: .evolve/inbox/2026-07-07T19-32-00Z-inbox-promotion-requires-
// landed-ship.json (weight 0.90). Incident: cycle-598 (batch b63fyf1ai) — ship
// push rejected, recovery ended needs-reaudit, cycle still reported PASS, and
// the inbox item was promoted to processed/ although the work never landed on
// any ref. Verdict is not delivery.
//
// AC map (1:1), derived from the inbox item's acceptance list. The landing
// gate itself landed in a prior cycle (postship.go isLanded + inboxmover
// IsLandedFn), so AC1 predicates pin PRE-EXISTING GREEN unit contracts; the
// residual RED gap this cycle is AC2's "returns with a retry note":
//
//	AC1 promotion refused when the ship commit is absent from main
//	    ancestry; twin: landed SHA promotes          → C752_001 + C752_002 (twin)
//	                                                   + C752_003 (unit-gate reroute)
//	AC2 reaudit-recovery terminal without landing releases the item
//	    back with a note                             → C752_004 (never promotes,
//	                                                   pre-existing GREEN)
//	                                                   + C752_005 (retry note, RED)
//	                                                   + C752_006 (negative anti-stamp)
//	AC3 go vet, -race, apicover -enforce green       → manual+checklist (auditor
//	    runs the repo-wide CI-parity gates on touched pkgs per ADR-0069)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit-test contract in the target package, which EXERCISES the SUT (scripted
// git runner seams, real temp inbox dirs, real ledger.jsonl writes) —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
package cycle752

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	moverPkg = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// runGoTest executes the named unit test under -race and requires an explicit
// verbose PASS marker so the predicate fails on: compile failure, test
// failure, a race report, a missing package, OR the test not existing
// (rename gaming).
func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

// AC1 (positive refusal) — an unlanded ship commit (merge-base --is-ancestor
// exit 1 for HEAD and origin) must never promote to processed/ and must log
// the unlanded WARN. Pre-existing GREEN pin.
func TestC752_001_UnlandedCommitNeverPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_UnlandedCommitSkipsPromotion")
}

// AC1 (twin — strongest anti-no-op signal): the exact same fixture with a
// LANDED SHA must still promote to processed/cycle-<cid>/. A gate that
// unconditionally refuses would pass C752_001 and must fail here.
func TestC752_002_LandedCommitPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_LandedCommitPromotes")
}

// AC1 (unit-gate layer) — inboxmover.Promote itself consults the IsLandedFn
// delivery-evidence seam: a processed-promotion carrying an unlanded SHA is
// rerouted away from processed/ with a ledgered reason. Pre-existing GREEN pin.
func TestC752_003_MoverReroutesUnlandedProcessedPromotion(t *testing.T) {
	runGoTest(t, moverPkg, "TestPromote_ProcessedRefusedWhenNotLanded")
}

// AC2 (refusal half) — the cycle-598 regression shape itself:
// RepairOutcome=="needs-reaudit" with an unlanded commit never promotes,
// regardless of any upstream PASS verdict. Pre-existing GREEN pin.
func TestC752_004_NeedsReauditTerminalNeverPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_NeedsReauditOutcomeNeverPromotes")
}

// AC2 (note half — the cycle-752 RED anchor) — an unlanded ship releases the
// item back to the inbox root AND leaves a durable per-item unlanded retry
// note in the lifecycle ledger, so triage/operators can tell a delivery
// failure from an ordinary residual drain.
func TestC752_005_UnlandedReleaseCarriesRetryNote(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_UnlandedReleaseCarriesRetryNote")
}

// AC2 (negative anti-stamp guard) — a LANDED cycle's ordinary residual drain
// must keep the generic release reason. A stub stamping "unlanded" on every
// release would pass C752_005 and must fail here.
func TestC752_006_LandedResidualDrainKeepsGenericReason(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_LandedResidualReleaseKeepsGenericReason")
}
