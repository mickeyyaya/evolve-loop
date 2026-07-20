//go:build acs

// Package cycle976 materialises the cycle-976 acceptance criteria for the two
// triage-committed top_n tasks (see scout-report.md / triage-report.md), both
// tracing to ONE wiring defect: Orchestrator.profileForModelRouting is a
// permanent nil-stub (cyclerun.go:711-713), so router.ClampPlanModelRouting's
// model-tier-envelope guard — floor, ceiling, AND the documented "universal
// floor" — never fires in the composed production path.
//
//   - wire-real-profiles-into-model-tier-envelope-guard: replace the nil-stub
//     with a real per-phase profile lookup so an out-of-envelope advisor tier
//     actually clamps end-to-end (TestC976_001, 003).
//   - universal-floor-wiring-proof-regression-test: prove the compiled
//     universalTierFloor fires through the real dispatch path, not just the
//     router unit boundary (TestC976_002).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574 precedent).
// Each predicate shells `go test -run` over the RED integration tests authored
// this cycle in internal/core (model_routing_envelope_wiring_test.go). None is a
// source-grep — every one exercises the system under test (a full RunCycle over
// a real Orchestrator with on-disk .evolve/profiles fixtures) and asserts on the
// tier the build phase is actually dispatched with. RED now: the nil-stub leaves
// the advisor tier unclamped. GREEN once Builder wires the real lookup.
package cycle976

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure or
// an assertion failure in the target package surfaces as a non-zero exit — the
// intended RED signal before Builder wires the seam. code < 0 is a genuine
// launch failure (binary missing / killed), never a test verdict, so it is a
// hard error rather than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC976_001_EnvelopeCeilingClampsThroughRealSeam — Task-1 wiring proof
// (ceiling / explicit envelope): a phase whose profile declares
// model_tier_envelope {min:balanced,max:deep} must clamp an above-ceiling "top"
// advisor proposal DOWN to "deep" when driven through the REAL
// Orchestrator.profileForModelRouting seam. RED now (nil-stub leaves "top"
// unclamped at dispatch); GREEN once the real profile lookup is wired.
func TestC976_001_EnvelopeCeilingClampsThroughRealSeam(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestModelTierEnvelope_CeilingClampsThroughRealProfileLookup")
	if !ok {
		t.Errorf("model-tier-envelope ceiling does not clamp through the real profileForModelRouting seam — the nil-stub is still disabling the guard in production:\n%s", out)
	}
}

// TestC976_002_UniversalFloorFiresInRealDispatch — Task-2 wiring proof
// (universal floor): an envelope-LESS profile must still clamp a below-floor
// "fast" proposal UP to the compiled universalTierFloor.Min ("balanced") in the
// composed production dispatch path, not merely at the router unit boundary. RED
// now (prof==nil skips the universal-floor substitution entirely).
func TestC976_002_UniversalFloorFiresInRealDispatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestModelTierEnvelope_UniversalFloorClampsThroughRealDispatch")
	if !ok {
		t.Errorf("the documented universal envelope floor does not fire in the real dispatch path — it is still gated behind the nil-profile stub:\n%s", out)
	}
}

// TestC976_003_ClampIsPreciseAndNilSafe — Task-1 anti-no-op + nil-safety guards.
// Two invariants the wiring must preserve: (a) a within-envelope tier passes
// through UNCLAMPED (the fix must not degenerate into "always clamp"), and (b) a
// phase with NO profile on disk degrades nil-safe (no clamp, no error), matching
// ValidatePin's nil-profile pass-through contract. These hold on both the
// nil-stub and the wired path, so they pin the fix's precision — catching a
// Builder over-clamp or a nil-panic regression. Also asserts the ceiling clamp
// is persisted to phase-plan.json (operator-visible evidence, RED now).
func TestC976_003_ClampIsPreciseAndNilSafe(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestModelTierEnvelope_WithinEnvelopeTierPassesThrough|TestModelTierEnvelope_AbsentProfileDegradesNilSafe|TestModelTierEnvelope_ClampRecordedInPhasePlan")
	if !ok {
		t.Errorf("envelope wiring is imprecise (over-clamps a legal tier), nil-unsafe (panics/errors on a profile-less phase), or does not record the clamp to phase-plan.json:\n%s", out)
	}
}

// TestC976_004_RouterUnitGuardUnchanged — Task-1 regression AC (pre-existing
// GREEN): the existing router-level model-tier-envelope guard tests still pass
// unchanged. The wiring closes the DI seam in core WITHOUT touching the guard
// logic in router — this predicate fails loudly if Builder regresses the
// router package while wiring core. Guards the "existing suite still passes
// unchanged" acceptance criterion.
func TestC976_004_RouterUnitGuardUnchanged(t *testing.T) {
	ok, out := runGoTest(t, routerPkg, "TestClampPlanModelRouting.*|.*ModelRouting.*Envelope.*|.*UniversalFloor.*")
	if !ok {
		t.Errorf("the router-level model-tier-envelope guard suite regressed — the core wiring must not alter router guard logic:\n%s", out)
	}
}

// TestC976_005_TreeBuildsNoImportCycle — Task-1 hard constraint: wiring a real
// per-phase profile lookup into core must NOT introduce an import cycle (the
// exact reason the original author left the stub). `go build ./...` over the
// whole module exits 0 only when no cycle exists. Behavioural: it compiles the
// real tree, not a source grep.
func TestC976_005_TreeBuildsNoImportCycle(t *testing.T) {
	_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "./...")
	if code < 0 {
		t.Fatalf("go build failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Errorf("go build ./... failed (exit=%d) — the profile-lookup wiring must not introduce an import cycle:\n%s", code, stderr)
	}
}
