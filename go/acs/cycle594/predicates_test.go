//go:build acs

// Package cycle594 materialises the cycle-594 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - memo-phase-tier-envelope (inbox 0.95 critical): (a) the memo phase's
//     model-tier pin must satisfy its profile's model_tier_envelope, config-only;
//     and (b) a PASS-side post-ship observer phase (memo / post-ship-monitor)
//     that fails AFTER a healthy ship must be non-fatal — degraded to a WARN
//     diagnostic, never a cycle-level failure that turns a shipped cycle abnormal.
//
// PRE-EXISTING GREEN (honest RED-phase note): this cycle is a goal-hash lane
// re-run of a task that already shipped to main. Commit 8ff4a85f (same
// goal_hash 805f6ced…) is an ancestor of HEAD; it landed BOTH halves — the
// config alignment (cycle-573: .evolve/profiles/memo.json envelope widened to
// min=fast so the fast pin, rank 1, sits inside [fast..balanced]) and the
// non-fatal post-ship classifier (cycle-574: Orchestrator.postShipObserverSkip,
// wired into cyclerun_dispatch.go). So the criteria below are ALREADY satisfied
// and these predicates run GREEN today rather than RED. They are retained as the
// cycle's audit-gating contract: each exercises the real system under test (not
// a source grep) and would go RED if the shipped config or classifier regressed.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…573 precedent).
// Each predicate shells `go test -run` over the internal-package unit tests that
// materialise the criteria — ValidatePin over the SHIPPED config, and the real
// Orchestrator.postShipObserverSkip classifier over a real catalog/cfg. None is
// a source-grep.
package cycle594

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or
// assertion failure in the target package surfaces as a non-zero exit — the RED
// signal a regression would produce.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2) must flow through as
	// ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC594_001_MemoPinWithinEnvelope — AC-2 (tier satisfies policy resolution):
// the shipped memo pin's model tier satisfies (and lands inside the rank band
// of) the memo profile's model_tier_envelope. Drives the shipped-config
// resolver tests in internal/policy — a live cycle's memo dispatch runs the
// exact ValidatePin check; a non-nil error here is the "outside envelope"
// abnormal this task eliminates.
func TestC594_001_MemoPinWithinEnvelope(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope drift not resolved in shipped config (config-only fix, no Go literals):\n%s", out)
	}
}

// TestC594_002_EnvelopeEnforcementIntact — AC-3 anti-no-op: the config fix must
// not be achieved by gutting envelope enforcement; a fast pin under a
// balanced-only envelope must still be rejected. Drives the negative test in
// internal/policy. This is the strongest anti-no-op signal — it fails only if
// someone "resolves" the drift by weakening ValidatePin instead of aligning
// config.
func TestC594_002_EnvelopeEnforcementIntact(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("envelope enforcement gutted — ValidatePin no longer rejects an out-of-envelope pin:\n%s", out)
	}
}

// TestC594_003_PostShipObserverNonFatal — AC-1 (the core regression): a memo
// failure on an ALREADY-SHIPPED cycle is classified non-fatal (degrade to WARN +
// advance) so the cycle keeps its shipped/PASS outcome and the lane reports ok.
// Drives the real Orchestrator.postShipObserverSkip classifier over a real
// catalog/cfg (internal/core) — the positive case plus the floor/ship guards
// that keep the downgrade from weakening the integrity floor.
func TestC594_003_PostShipObserverNonFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestPostShipObserverSkip_MemoAfterShipIsNonFatal|TestPostShipObserverSkip_MandatoryFloorNeverSkipped|TestPostShipObserverSkip_ShipItselfNeverSkipped")
	if !ok {
		t.Errorf("post-ship observer failure not classified non-fatal (a shipped cycle can still be turned abnormal by a memo error):\n%s", out)
	}
}

// TestC594_004_PreShipFailureStaysFatal — AC-1 negative / anti-no-op: the
// non-fatal downgrade is scoped to POST-ship. A memo failure when ship has NOT
// yet landed (shipped==false) must stay cycle-fatal — otherwise a broken memo
// could mask a genuine pre-ship failure. Drives the discriminator test in
// internal/core; forbids a degenerate "memo is always skipped" implementation.
func TestC594_004_PreShipFailureStaysFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestPostShipObserverSkip_MemoBeforeShipStaysFatal")
	if !ok {
		t.Errorf("pre-ship observer failure wrongly swallowed — the non-fatal downgrade must apply only once ship has landed:\n%s", out)
	}
}
