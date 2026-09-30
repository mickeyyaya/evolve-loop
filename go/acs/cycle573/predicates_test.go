//go:build acs

package cycle573

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	auditPkg       = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	dossierPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
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

func TestC573_001_MemoPinWithinEnvelope(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestMemoPin_WithinShippedEnvelope|TestMemoPin_TierRankMatchesEnvelope")
	if !ok {
		t.Errorf("memo pin/envelope drift not resolved in shipped config (config-only fix, no Go literals):\n%s", out)
	}
}

func TestC573_002_EnvelopeEnforcementIntact(t *testing.T) {
	ok, out := runGoTest(t, policyPkg, "TestValidatePin_StillRejectsOutOfEnvelope")
	if !ok {
		t.Errorf("envelope enforcement gutted — ValidatePin no longer rejects an out-of-envelope pin:\n%s", out)
	}
}

func TestC573_003_ChangedPkgsFromGit(t *testing.T) {
	ok, out := runGoTest(t, changedpkgsPkg, "TestFromGit_DetectsChangedGoPackage|TestFromGit_NoChangesEmpty|TestFromGit_IgnoresNonGoChanges")
	if !ok {
		t.Errorf("changedpkgs.FromGit missing or wrong — deterministic git-derived changed-package source not in place:\n%s", out)
	}
}

func TestC573_004_ApicoverGateNoLongerFailOpen(t *testing.T) {
	ok, out := runGoTest(t, auditPkg, "TestChangedPackagesForAudit_GitDerivedNoHandoff")
	if !ok {
		t.Errorf("changedPackagesForAudit still fail-open on missing handoff (apicover gate silently no-ops):\n%s", out)
	}
}

func TestC573_005_DossierRollbackOnPermanentFailure(t *testing.T) {
	ok, out := runGoTest(t, dossierPkg, "TestCommitPairGit_RollsBackStagedOnPermanentFailure")
	if !ok {
		t.Errorf("dossier commit does not roll back the staged pair on permanent failure (next-cycle tree-diff pollution):\n%s", out)
	}
}
