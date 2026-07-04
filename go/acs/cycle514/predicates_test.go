//go:build acs

// Package cycle514 materialises the cycle-514 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	boot-recovery-auto-repin-shipsha (bug, CRITICAL — 9-cycle SELF_SHA_TAMPERED
//	ship cascade, carryover cycles 498/500/502/508-513). Cycle 507 wired
//	*detection* of a ship-binary SHA mismatch into runLoop's boot path but only
//	WARNs — it never invokes the existing provenance-gated repin primitive
//	phaseintegrity.RepinShipSHA, so the cascade kept recurring. This task closes
//	the wiring gap: a provenance-VERIFIED mismatch (legit rebuild) auto-repins at
//	boot; an UNVERIFIED mismatch (possible tampering) is still refused.
//
// (task triage-worktree-leak-root-cause is DEFERRED — no predicates authored for
// it, per R9.3: predicates bind ONLY to triage-committed work.)
//
// Predicate strategy (mirrors cycle499/cycle503/cycle504/cycle507): BEHAVIORAL
// predicates drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan closes
// the cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer:
//
//	cmd/evolve/cmd_loop_boot_recovery_repin_test.go  (auto-repin wiring: positive/negative/edge)
//
// The Builder implements production code ONLY (the seam named in that file —
// bootRecoveryResult.Healed + shipRepinProvenanceFn + the RepinShipSHA call in
// defaultBootRecovery); it must not modify the tests.
package cycle514

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioral predicates invoke the system under test through
// its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test (renamed/unwritten) — or a package that fails to build — exits
// without running the required tests, which must NOT green the predicate.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

// TestC514_001_AutoRepinsWhenProvenanceVerified (AC-1, positive): a
// provenance-verified ship-SHA mismatch (a legitimate rebuild) is auto-repinned
// at boot, so the very next boot/ship sees no mismatch — the 508-513
// SELF_SHA_TAMPERED cascade is self-healed. Drives cmd/evolve
// cmd_loop_boot_recovery_repin_test.go. RED today: bootRecoveryResult.Healed and
// shipRepinProvenanceFn are undefined (package main test build fails).
func TestC514_001_AutoRepinsWhenProvenanceVerified(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_AutoRepinsWhenProvenanceVerified", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery does NOT auto-repin a provenance-verified SHA mismatch (exit=%d) — wire phaseintegrity.RepinShipSHA into defaultBootRecovery's mismatch branch\n%s", code, out)
	}
}

// TestC514_002_DeclinesRepinWhenProvenanceUnverified (AC-2, negative /
// anti-tamper): an UNVERIFIABLE mismatch (possible tampering) is NOT repinned —
// the mismatch stays flagged and the pin is untouched. This is the anti-no-op
// predicate: an unconditional repin (trust-kernel hole) fails here. Drives
// cmd/evolve cmd_loop_boot_recovery_repin_test.go. RED today: same build failure.
func TestC514_002_DeclinesRepinWhenProvenanceUnverified(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_DeclinesRepinWhenProvenanceUnverified", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery repins an UNVERIFIABLE binary (exit=%d) — anti-tamper broken; pass operatorAuthorized=false and refuse on provenance failure\n%s", code, out)
	}
}

// TestC514_003_NoExpectedSHAIsNoOpAndSkipsProvenance (AC-3, edge): a project with
// no expected_ship_sha yet is a no-op AND boot recovery never reaches the
// provenance/git path (short-circuit before any git subprocess). Drives
// cmd/evolve cmd_loop_boot_recovery_repin_test.go. RED today: same build failure.
func TestC514_003_NoExpectedSHAIsNoOpAndSkipsProvenance(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_NoExpectedSHAIsNoOpAndSkipsProvenance", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot recovery does not short-circuit on an absent expected_ship_sha (exit=%d) — it must not invoke the provenance/git path when there is nothing pinned\n%s", code, out)
	}
}
