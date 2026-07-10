//go:build acs

// Package cycle639 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 639, self-sha-fail-early-boot-gate
// (weight 0.95, inbox 2026-07-08T03-05-30Z-self-sha-fail-early-boot-gate.json;
// carryover, named in cycles 629/630/632 lessons, never built).
//
// Defect: a WITHIN-VERSION ship-binary SHA mismatch (state.json:
// expected_ship_version == the current plugin version, but expected_ship_sha !=
// the on-disk go/bin/evolve) is boot-time-knowable and cycle-fatal — it is
// exactly the SELF_SHA_TAMPERED integrity failure the terminal ship gate raises
// (internal/phases/ship/verify.go:119-127). Yet boot only WARNed and proceeded,
// so 8 consecutive cycles (625-634) each burned a full ~32-40 min lane before
// dying at the terminal ship gate on a ship doomed from boot.
//
// Fix: classify the mismatch at boot the SAME way verifySelfSHA does — a
// within-version mismatch HALTs pre-scout with the operator-unblock recipe
// (`make -C go build` → `evolve reset-sha -operator` → relaunch) and does NOT
// auto-repin; an across-version / legacy-unversioned mismatch stays on the
// existing cycle-514 boot auto-repin path unchanged; a matching SHA boots into
// scout.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…637 precedent).
// Each predicate shells `go test -run` over the RED regression tests authored
// this cycle in cmd/evolve/cmd_loop_boot_selfsha_gate_test.go. Every one
// EXERCISES the system under test — the real defaultBootRecovery (driven with a
// real on-disk state.json + go/bin/evolve + .claude-plugin/plugin.json fixture)
// and the real runLoop boot path — and asserts on the resulting behaviour
// (HaltSelfSHA set + pin untouched + recipe emitted / auto-repin still fires for
// across-version / no action on a matched SHA / runLoop returns 2 pre-preflight).
// None is a source-grep. RED now: the cmd/evolve test package fails to build
// because bootRecoveryResult has no HaltSelfSHA field. GREEN once Builder adds
// the field, the within-version classification + halt-recipe in
// defaultBootRecovery, and the pre-scout return in runLoop.
//
// The fourth Acceptance line ("go vet ./..., -race, apicover green") is
// dispositioned manual+checklist in test-report.md — a repo-wide toolchain gate
// the cycle audit already runs, not predicated here.
package cycle639

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. code<0 is a genuine
// launch failure (binary missing / killed by signal), never a test verdict —
// that fails loudly rather than being misread as a RED behavioural result.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC639_001_HaltsPreScoutOnWithinVersionMismatch — AC1 (headline): a
// within-version ship-SHA mismatch HALTs boot pre-scout with the operator recipe
// and does NOT auto-repin, AND runLoop returns 2 before the readiness gate is
// reached (no scout phase runs). Shells BOTH the unit-level classification proof
// and the runLoop integration proof so the whole "halt pre-scout, no scout" claim
// is exercised end-to-end.
func TestC639_001_HaltsPreScoutOnWithinVersionMismatch(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg,
		"TestBootGate_HaltsOnWithinVersionSelfShaMismatch|TestRunLoop_HaltsPreScoutOnWithinVersionSelfShaMismatch")
	if !ok {
		t.Errorf("within-version ship-SHA mismatch does not HALT boot pre-scout with the operator recipe:\n%s", out)
	}
}

// TestC639_002_AcrossVersionStillAutoRepins — AC2 (regression twin): an
// across-version mismatch (a legitimate plugin/version bump) stays on the
// existing cycle-514 boot auto-repin path — it heals and re-pins the SHA and
// does NOT trip the new within-version halt. Guards the fix against regressing
// the established auto-repin behavior.
func TestC639_002_AcrossVersionStillAutoRepins(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestBootGate_AcrossVersionMismatchStillAutoRepins")
	if !ok {
		t.Errorf("across-version mismatch no longer auto-repins at boot (existing behavior regressed):\n%s", out)
	}
}

// TestC639_003_MatchingSHABootsIntoScout — AC3 (negative / matched tree): when
// the on-disk binary SHA matches expected_ship_sha under the same plugin
// version, boot takes zero self-SHA action (no halt, no flag, no heal) and falls
// through to scout — the gate must not halt spuriously on a healthy tree.
func TestC639_003_MatchingSHABootsIntoScout(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestBootGate_MatchingSHABootsIntoScout")
	if !ok {
		t.Errorf("a matched-SHA healthy tree does not boot cleanly into scout (spurious halt or action):\n%s", out)
	}
}
