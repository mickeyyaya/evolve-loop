//go:build acs

// Package cycle574 materialises the cycle-574 acceptance criteria for the single
// triage-committed top_n task (see scout-report.md / triage-report.md):
//
//   - fix-memo-phase-tier-envelope (inbox 0.95 critical) — the memo phase turns
//     a healthy PASS cycle abnormal post-ship. This has TWO halves:
//     (1) TIER: align the memo pin's model tier with its profile envelope so
//     ValidatePin stops raising "outside envelope". Landed in cycle-573
//     (internal/policy memo_envelope_config_test.go) — GREEN; guarded here
//     as REGRESSION coverage (TestC574_001).
//     (2) NON-FATAL OBSERVER: a memo failure AFTER a healthy ship must degrade
//     to a WARN diagnostic, never a cycle-level failure. This is the
//     still-unimplemented half and the RED work of cycle-574
//     (TestC574_002..004, driving the new internal/core classifier).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…573 precedent).
// Each predicate shells `go test -run` over the RED unit tests authored this
// cycle in internal/core (postShipObserverSkip) plus the cycle-573 regression
// tests in internal/policy. None is a source-grep — every one exercises the
// system under test (postShipObserverSkip over a real orchestrator catalog/cfg;
// ValidatePin over the shipped config) and asserts on its result. RED now:
// internal/core fails to compile (postShipObserverSkip undefined). GREEN once
// Builder adds the classifier and wires it into the dispatch abort path.
package cycle574

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	policyPkg = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (e.g. undefined postShipObserverSkip) surfaces as a
// non-zero exit — the intended RED signal before Builder implements.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	// code < 0 is a genuine launch failure (binary missing / killed by signal),
	// not a test verdict; SubprocessOutput returns non-nil err for ANY non-zero
	// exit, so a plain compile/assertion failure (code 1/2 — the RED signal)
	// must flow through as ok=false, NOT be misread as "failed to launch".
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC574_001_MemoTierEnvelopeAligned — AC-1 (regression, pre-existing GREEN):
// the shipped memo pin's model tier still satisfies (and lands inside the rank
// band of) its profile envelope, and envelope enforcement is not gutted. Drives
// the cycle-573 shipped-config tests in internal/policy. Guards against a
// regression of the tier half while cycle-574 adds the observer half.
func TestC574_001_MemoTierEnvelopeAligned(t *testing.T) {
	ok, out := runGoTest(t, policyPkg,
		"TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope|TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope alignment regressed (config-only fix, no Go literals):\n%s", out)
	}
}

// TestC574_002_MemoPostShipFailureNonFatal — AC-2a (core RED): a memo failure on
// an ALREADY-SHIPPED cycle is classified non-fatal (postShipObserverSkip==true),
// so the shipped/PASS outcome survives. RED now: postShipObserverSkip is
// undefined → internal/core does not compile. Drives the classifier test.
func TestC574_002_MemoPostShipFailureNonFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestPostShipObserverSkip_MemoAfterShipIsNonFatal")
	if !ok {
		t.Errorf("post-ship memo failure still flips a shipped cycle abnormal — the non-fatal observer classifier is missing:\n%s", out)
	}
}

// TestC574_003_PostShipSkipScopedAndFloorSafe — AC-2b/2c/2d (anti-no-op +
// floor/anchor guards): the downgrade is scoped to POST-ship (a pre-ship memo
// failure stays fatal), and a mandatory/floor phase and ship itself are never
// skipped. Proves the fix is not a degenerate "always swallow memo" and never
// weakens the integrity floor. Drives the negative + guard classifier tests.
func TestC574_003_PostShipSkipScopedAndFloorSafe(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestPostShipObserverSkip_MemoBeforeShipStaysFatal|TestPostShipObserverSkip_MandatoryFloorNeverSkipped|TestPostShipObserverSkip_ShipItselfNeverSkipped")
	if !ok {
		t.Errorf("post-ship observer skip is mis-scoped (swallows pre-ship failures, or skips a floor/anchor phase):\n%s", out)
	}
}
