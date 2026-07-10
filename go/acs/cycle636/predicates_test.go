//go:build acs

// Package cycle636 materialises the cycle-636 acceptance criteria for the single
// triage-committed top_n task, ship-sha-repin-after-build (weight 0.96, inbox
// 2026-07-08T03-05-00Z-ship-sha-repin-after-build.json). SELF_SHA_TAMPERED denied
// the terminal ship gate on 8 consecutive cycles (625->634) with a byte-identical
// FROZEN pin, both within plugin version 22.0.1: the cycle-514 boot healer re-pins
// only at boot, so a legitimate within-version rebuild of go/bin/evolve leaves a
// doomed pin for every later cycle. The fix runs the SAME provenance-gated repin
// immediately AFTER a successful build phase, reusing one shared primitive
// (phaseintegrity.RepinIfDrifted) so the boot and post-build paths never diverge.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…623 precedent) —
// each predicate shells `go test -run` over the RED unit tests authored this cycle
// (with -count=1 to defeat the test cache, so it always exercises current source).
// None is a source-grep; every one exercises the system under test — the shared
// primitive phaseintegrity.RepinIfDrifted and the orchestrator entry
// repinShipSHAAfterBuild, each called with real on-disk state.json + binary
// fixtures and asserted on their RepinResult / the resulting pin / ShipSHAMismatch.
// RED now: internal/phaseintegrity and internal/core both fail to compile
// (RepinIfDrifted / repinShipSHAAfterBuild / postBuildRepinProvenanceFn undefined).
// GREEN once Builder implements the primitive + the post-build entry and wires it
// into recordAndBranch's PhaseBuild branch (test-report.md WIR-1).
//
// The recordAndBranch call-site wiring (WIR-1) is dispositioned manual+checklist
// in test-report.md — a full-cycle recordAndBranch run is disproportionately heavy
// to unit-drive; the entry function's behaviour is fully predicated here and AC-3
// (TestC636_004) is the mechanical ship-gate backstop.
package cycle636

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phaseintegrityPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	corePkg           = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. code<0 is a genuine
// launch failure (binary missing / killed by signal), never a test verdict — that
// must fail loudly, not be misread as RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC636_001_RepinsAfterBuildNotJustBoot — AC-1 (the acceptance's named RED
// test): a provenance-verified in-version rebuild is re-pinned AFTER the build,
// not only at boot, so expected_ship_sha tracks the freshly-built binary.
func TestC636_001_RepinsAfterBuildNotJustBoot(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestBootRecovery_RepinsAfterBuildNotJustBoot")
	if !ok {
		t.Errorf("post-build repin does not fire for a provenance-verified in-version rebuild:\n%s", out)
	}
}

// TestC636_002_SharedPrimitiveRepinsOnVerifiedDrift — AC-1 (primitive): the shared
// phaseintegrity.RepinIfDrifted re-pins on a verified-provenance drift, proving
// boot + post-build ride ONE centralized repin path (never duplicated).
func TestC636_002_SharedPrimitiveRepinsOnVerifiedDrift(t *testing.T) {
	ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_ProvenanceVerifiedRebuild_Repins")
	if !ok {
		t.Errorf("shared RepinIfDrifted primitive does not re-pin on verified drift:\n%s", out)
	}
}

// TestC636_003_UnverifiedProvenanceNeverRepinned — AC-2 (twin / anti-tamper): a
// provenance-UNVERIFIED binary change is refused at BOTH layers (the shared
// primitive AND the orchestrator entry). The pin is left untouched; tamper
// detection is not weakened. Strongest anti-no-op signal — an unconditional repin
// fails both halves.
func TestC636_003_UnverifiedProvenanceNeverRepinned(t *testing.T) {
	if ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_UnverifiedProvenance_RefusesAndKeepsPin"); !ok {
		t.Errorf("RepinIfDrifted re-pinned an UNVERIFIED binary (trust-kernel hole):\n%s", out)
	}
	if ok, out := runGoTest(t, corePkg, "TestBootRecovery_PostBuildRepin_UnverifiedProvenance_KeepsPin"); !ok {
		t.Errorf("post-build repin re-pinned an UNVERIFIED binary (trust-kernel hole):\n%s", out)
	}
}

// TestC636_004_AfterBuildRepinLeavesNoShipSHAMismatch — AC-3 (verify-only no-op
// cycle ships): the mechanical proof that after the post-build repin the ship
// gate's own detector (core.ShipSHAMismatch) sees NO mismatch — a verify-only
// cycle on the freshly-rebuilt binary would not be denied SELF_SHA_TAMPERED.
func TestC636_004_AfterBuildRepinLeavesNoShipSHAMismatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestBootRecovery_AfterBuildRepin_ShipGateSeesNoMismatch")
	if !ok {
		t.Errorf("after post-build repin the ship gate still sees a SHA mismatch (cascade not fixed):\n%s", out)
	}
}

// TestC636_005_NoDriftAndMissingPinAreNoOps — edge safety: a no-drift binary, a
// project with no pin, and a project with no built binary are all fail-open no-ops
// (no spurious re-pin, no panic) so the hook is safe to run every cycle.
func TestC636_005_NoDriftAndMissingPinAreNoOps(t *testing.T) {
	if ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_NoDrift_IsNoOp|TestRepinIfDrifted_MissingPinOrBinary_IsNoOp"); !ok {
		t.Errorf("RepinIfDrifted no-op edge cases (no drift / no pin / no binary) are not safe no-ops:\n%s", out)
	}
	if ok, out := runGoTest(t, corePkg, "TestBootRecovery_PostBuildRepin_NoBinaryIsNoOp"); !ok {
		t.Errorf("post-build repin is not a safe no-op when the binary is absent:\n%s", out)
	}
}

// TestC636_006_AffectedPackagesVetClean — AC-4 (go vet green on affected pkgs):
// vet compiles + statically checks internal/phaseintegrity and internal/core
// (including this cycle's new test files). RED now (undefined symbols → build
// error); GREEN once the primitive + entry land.
func TestC636_006_AffectedPackagesVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", phaseintegrityPkg, corePkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go vet failed to launch: code=%d err=%v\n%s", code, err, out)
	}
	if code != 0 {
		t.Errorf("go vet is not clean on the affected packages:\n%s", out)
	}
}

// TestC636_007_RepinIfDriftedNamedForApicover — AC-4 (apicover green): the new
// exported primitive RepinIfDrifted is referenced by the phaseintegrity apicover
// named-API test, so the public-API coverage gate stays green.
func TestC636_007_RepinIfDriftedNamedForApicover(t *testing.T) {
	ok, out := runGoTest(t, phaseintegrityPkg, "TestNamePublicAPI_RepinIfDrifted")
	if !ok {
		t.Errorf("RepinIfDrifted is not covered by the apicover named-API test:\n%s", out)
	}
}
