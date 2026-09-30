//go:build acs

package cycle636

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phaseintegrityPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	corePkg           = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC636_001_RepinsAfterBuildNotJustBoot(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestBootRecovery_RepinsAfterBuildNotJustBoot")
	if !ok {
		t.Errorf("post-build repin does not fire for a provenance-verified in-version rebuild:\n%s", out)
	}
}

func TestC636_002_SharedPrimitiveRepinsOnVerifiedDrift(t *testing.T) {
	ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_ProvenanceVerifiedRebuild_Repins")
	if !ok {
		t.Errorf("shared RepinIfDrifted primitive does not re-pin on verified drift:\n%s", out)
	}
}

func TestC636_003_UnverifiedProvenanceNeverRepinned(t *testing.T) {
	if ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_UnverifiedProvenance_RefusesAndKeepsPin"); !ok {
		t.Errorf("RepinIfDrifted re-pinned an UNVERIFIED binary (trust-kernel hole):\n%s", out)
	}
	if ok, out := runGoTest(t, corePkg, "TestBootRecovery_PostBuildRepin_UnverifiedProvenance_KeepsPin"); !ok {
		t.Errorf("post-build repin re-pinned an UNVERIFIED binary (trust-kernel hole):\n%s", out)
	}
}

func TestC636_004_AfterBuildRepinLeavesNoShipSHAMismatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestBootRecovery_AfterBuildRepin_ShipGateSeesNoMismatch")
	if !ok {
		t.Errorf("after post-build repin the ship gate still sees a SHA mismatch (cascade not fixed):\n%s", out)
	}
}

func TestC636_005_NoDriftAndMissingPinAreNoOps(t *testing.T) {
	if ok, out := runGoTest(t, phaseintegrityPkg, "TestRepinIfDrifted_NoDrift_IsNoOp|TestRepinIfDrifted_MissingPinOrBinary_IsNoOp"); !ok {
		t.Errorf("RepinIfDrifted no-op edge cases (no drift / no pin / no binary) are not safe no-ops:\n%s", out)
	}
	if ok, out := runGoTest(t, corePkg, "TestBootRecovery_PostBuildRepin_NoBinaryIsNoOp"); !ok {
		t.Errorf("post-build repin is not a safe no-op when the binary is absent:\n%s", out)
	}
}

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

func TestC636_007_RepinIfDriftedNamedForApicover(t *testing.T) {
	ok, out := runGoTest(t, phaseintegrityPkg, "TestNamePublicAPI_RepinIfDrifted")
	if !ok {
		t.Errorf("RepinIfDrifted is not covered by the apicover named-API test:\n%s", out)
	}
}
