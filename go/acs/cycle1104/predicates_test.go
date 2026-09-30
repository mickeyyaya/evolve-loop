//go:build acs

package cycle1104

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	continuationPkg = "github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	inboxmoverPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	corePkg         = "github.com/mickeyyaya/evolve-loop/go/internal/core"
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

func TestC1104_001_RegistryBindsScopeIDsToOneSchema(t *testing.T) {
	ok, out := runGoTest(t, continuationPkg,
		"TestRegistry_RoundTripByScopeID|TestRegistry_SecondScopeDoesNotClobberFirst|TestRegistry_StoresTheContinuationSchemaVerbatim")
	if !ok {
		t.Errorf("the scope-id-keyed continuation registry does not round-trip, isolates scopes, "+
			"or has forked the Continuation schema:\n%s", out)
	}
}

func TestC1104_002_RegistryAbsenceIsCleanAndCorruptionIsLoud(t *testing.T) {
	ok, out := runGoTest(t, continuationPkg,
		"TestRegistry_MissingFileAndUnknownScopeAreCleanMiss|TestRegistry_CorruptFileIsLoudError")
	if !ok {
		t.Errorf("registry absence is not a clean miss, or a corrupt registry is silently "+
			"swallowed instead of surfacing loudly:\n%s", out)
	}
}

func TestC1104_003_ResolveFallsBackToLaneScopeWithClaimFirst(t *testing.T) {
	ok, out := runGoTest(t, inboxmoverPkg,
		"TestResolveContinuationForScope_FallsBackToLaneScopeRegistry|TestResolveContinuationForScope_ClaimWinsOverRegistry|TestResolveContinuationForScope_UnstampedClaimStillFallsBack|TestResolveContinuationForScope_ScopeOrderIsDeterministic")
	if !ok {
		t.Errorf("lane-scope continuation resolution is missing, or it no longer tries the "+
			"inbox claim first (G1/PR #363 regression):\n%s", out)
	}
}

func TestC1104_004_ResolveNeverInventsOrLeaksABinding(t *testing.T) {
	ok, out := runGoTest(t, inboxmoverPkg,
		"TestResolveContinuationForScope_NoBindingIsNil|TestResolveContinuationForScope_EmptySnapshotIsNotABinding|TestResolveContinuationForScope_CorruptRegistryIsNilNotPanic|TestResolveContinuation_ClaimOnlyPathUnchanged")
	if !ok {
		t.Errorf("scope resolution invents a binding for an unrelated/blank scope, accepts an "+
			"empty snapshot ref, panics on a corrupt registry, or leaked the fallback into the "+
			"claim-only entry point:\n%s", out)
	}
}

func TestC1104_005_FailReleaseRegistersLaneScopeBindingOnTheCleanGate(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestStampContinuationManifest_RegistersLaneScopeBinding|TestStampContinuationManifest_RegistersEveryLaneScopeID|TestStampContinuationManifest_NoLaneScopeRegistersNothing|TestStampContinuationManifest_UnstampableWorkRegistersNothing")
	if !ok {
		t.Errorf("the preserve decision does not register a lane-scope binding, registers work "+
			"the carry-forward screen rejected (unresumable binding), or mints a binding for a "+
			"cycle that has no lane scope:\n%s", out)
	}
}

func TestC1104_006_NonClaimCycleAdoptsPreservedWorkEndToEnd(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim|TestRunCycle_UnrelatedLaneScopeDoesNotAdopt|TestAdoptContinuation_PassesLaneScopeIDsToResolver")
	if !ok {
		t.Errorf("a non-claim lane still cannot adopt its own preserved work (the cycle-1078 "+
			"orphan class is open), or it adopts another lane's:\n%s", out)
	}
}

func TestC1104_007_ExistingContinuationContractStillGreen(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"TestSnapshotPreservedWorktree_.*|TestStampContinuationManifest_WritesGatedManifest|TestStampContinuationManifest_ConflictingWorkIsNotStamped|TestValidateContinuation_.*|TestGitWorktree_CreateFromSeedsSnapshot|TestRunCycle_AdoptsContinuationAndServesFindings|TestRunCycle_InvalidContinuationFallsBackFresh"); !ok {
		t.Errorf("the ADR-0076 slice C claim-path contract regressed in core:\n%s", out)
	}
	if ok, out := runGoTest(t, inboxmoverPkg,
		"TestReleaseCycleProcessing_StampsContinuationFromManifest|TestReleaseCycleProcessing_NoManifestNoStamp|TestQuarantinePromotion_ShedsContinuationStamp|TestResolveContinuation_FirstStampedClaimWins"); !ok {
		t.Errorf("the ADR-0076 slice C claim-path contract regressed in the inbox mover:\n%s", out)
	}
}
