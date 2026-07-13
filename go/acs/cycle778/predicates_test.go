//go:build acs

// Package cycle778 materializes the cycle-778 acceptance criteria for the sole
// committed top_n task ship-window-lease (triage-report.md ## top_n; this
// lane's fleet_scope assigns exactly that id, so per R9.3 no predicates bind
// to the scout report's other-lane findings).
//
// Task source: inbox id ship-window-lease (weight 0.97, operator-boosted
// 2026-07-13, campaign tokenopt-2026-07). Measured cycles 767-774: audit ran
// ~10x for 8 cycles — AUDIT_BINDING_HEAD_MOVED re-audits from siblings landing
// on main between a lane's audit-binding snapshot and its push. The fix is a
// ship-window lease (go/internal/shipwindow) serializing ONLY the
// binding-snapshot→push section, with TTL + holder-death recovery (run-lease
// liveness pattern) and FIFO fairness.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 sibling waits instead of re-auditing (two lanes, one main HEAD,
//	    zero AUDIT_BINDING_HEAD_MOVED)
//	    → C778_001 (mutual exclusion + zero head-moved + both lanes ship),
//	      C778_002 (NEGATIVE: a fresh live-holder lease blocks a sibling
//	      until its ctx expires — the anti-no-op predicate).
//	AC2 holder death recovered: stale lease broken (dead pid before TTL;
//	    TTL expiry despite live pid) → C778_003.
//	AC3 FIFO fairness among queued waiters → C778_004.
//	AC4 batch soak (audit runs ≈ cycle count) + go test -race + apicover
//	    → soak is manual+checklist (test-report.md, addressed to Auditor);
//	      -race is exercised by every predicate below; apicover runs in the
//	      repo-wide gate. C778_005 pins the on-disk lease path contract.
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// shipwindow unit contract, which EXERCISES Acquire/Release behaviorally — no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects
// a rename/no-tests-matched silent green. Adversarial axes: negative (held
// lease must BLOCK, not yield), edge (dead-pid-within-TTL and
// TTL-expired-live-pid boundary breaks), semantic (exclusion vs recovery vs
// fairness vs path are separate behaviors).
package cycle778

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipwindowPkg = "github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"

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

// AC1: two lanes racing one main HEAD through the binding-snapshot→push
// section produce ZERO AUDIT_BINDING_HEAD_MOVED events and both still ship.
func TestC778_001_sibling_waits_instead_of_reaudit(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_SiblingWaitsInsteadOfReaudit")
}

// AC1 negative (anti-no-op): a fresh live-holder lease BLOCKS a sibling's
// Acquire until the sibling's context expires — a stub lease that always
// grants would fail this predicate.
func TestC778_002_held_lease_blocks_sibling(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_HeldLeaseBlocksSibling")
}

// AC2 edge: a stale lease is broken on BOTH boundaries — holder pid dead while
// the heartbeat is still within TTL, and heartbeat aged past TTL while the pid
// is still alive (hung holder).
func TestC778_003_holder_death_recovered(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_HolderDeathRecovered")
}

// AC3: queued waiters acquire in FIFO order of their Acquire calls.
func TestC778_004_fifo_fairness(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_FIFOFairness")
}

// AC4 (path contract): the lease file lives at <evolveDir>/ship-window.lock —
// the identity operators grep and gc sweeps.
func TestC778_005_lease_path_contract(t *testing.T) {
	runGoTest(t, shipwindowPkg, "TestShipWindowLease_PathIn")
}
