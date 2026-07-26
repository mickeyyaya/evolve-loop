//go:build acs

// Package cycle1104 materialises the cycle-1104 acceptance criteria for the
// single triage-committed top_n task of this lane:
// `continuation-nonclaim-scope-binding` (ADR-0076 slice C, G2).
//
// The defect: continuation bindings key ONLY off inbox-claimed processing
// scopes (.evolve/inbox/processing/cycle-N/*.json). Cycle-1078's failing lane
// (`chain-boundary-loop`) took its scope from the wave planner, so there was no
// item file for the FAIL-release path to stamp — the preserved snapshot was
// orphaned and no later attempt could ever adopt it. G2 adds the second
// scope-identity class: lane-scope todo ids (the authoritative
// <workspace>/lane-scope.json pin), reusing internal/continuation.Continuation
// VERBATIM (no forked schema), with claim resolution still tried first so
// G1/PR #363 semantics are untouched.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1102
// precedent). The produce side is an UNEXPORTED method on core.Orchestrator
// (stampContinuationManifest) and the adoption seam is unexported plumbing, so
// neither can be imported from here; each predicate instead shells
// `go test -run` over the RED contract tests authored this cycle in
// go/internal/{continuation,inboxmover,core}. Every one of those exercises the
// system under test — driving the real registry writer/reader, the real
// resolver, the real stamp method over a real git fixture, and RunCycle
// end-to-end through the production resolver closure — and asserts on returned
// values, on-disk artifacts and dispatched phase requests. None is a source-grep
// of production code (the cycle-85 degenerate-predicate ban).
//
// RED now: WriteRegistryEntry / ReadRegistryEntry / RegistryPath /
// ResolveContinuationForScope do not exist and WithContinuationResolver still
// takes the 2-arg closure, so all three packages fail to compile.
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

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (the RED signal before Builder implements) surfaces as a
// non-zero exit.
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

// TestC1104_001_RegistryBindsScopeIDsToOneSchema — AC2 (single schema) plus the
// registry's storage contract. Drives WriteRegistryEntry/ReadRegistryEntry over
// real temp-dir roots: round-trip fidelity, per-scope isolation under a
// read-modify-write (a whole-file overwrite would silently orphan every other
// lane's binding), and byte-level proof that the stored value is
// continuation.Continuation verbatim — same json field set as the manifest, no
// forked format.
func TestC1104_001_RegistryBindsScopeIDsToOneSchema(t *testing.T) {
	ok, out := runGoTest(t, continuationPkg,
		"TestRegistry_RoundTripByScopeID|TestRegistry_SecondScopeDoesNotClobberFirst|TestRegistry_StoresTheContinuationSchemaVerbatim")
	if !ok {
		t.Errorf("the scope-id-keyed continuation registry does not round-trip, isolates scopes, "+
			"or has forked the Continuation schema:\n%s", out)
	}
}

// TestC1104_002_RegistryAbsenceIsCleanAndCorruptionIsLoud — NEGATIVE half of
// the registry contract. A repo that never stamped a non-claim continuation, an
// unknown scope, and a blank scope id are all clean misses (never a phantom
// binding, never an error that would break ordinary cycles); a present-but-
// unparseable registry is a LOUD error, mirroring ReadManifest's schema-drift
// rule — silent recovery would hide the very orphan class this slice closes.
func TestC1104_002_RegistryAbsenceIsCleanAndCorruptionIsLoud(t *testing.T) {
	ok, out := runGoTest(t, continuationPkg,
		"TestRegistry_MissingFileAndUnknownScopeAreCleanMiss|TestRegistry_CorruptFileIsLoudError")
	if !ok {
		t.Errorf("registry absence is not a clean miss, or a corrupt registry is silently "+
			"swallowed instead of surfacing loudly:\n%s", out)
	}
}

// TestC1104_003_ResolveFallsBackToLaneScopeWithClaimFirst — AC1 + AC3, the
// resolve side. Drives ResolveContinuationForScope over real fixtures: a cycle
// with NO processing claim at all resolves its lane-scope binding (the
// cycle-1078 case), a claim-stamped continuation still WINS over the registry
// (G1 untouched), an UNSTAMPED claim does not suppress the fallback, and
// multi-id lanes resolve in declared order.
func TestC1104_003_ResolveFallsBackToLaneScopeWithClaimFirst(t *testing.T) {
	ok, out := runGoTest(t, inboxmoverPkg,
		"TestResolveContinuationForScope_FallsBackToLaneScopeRegistry|TestResolveContinuationForScope_ClaimWinsOverRegistry|TestResolveContinuationForScope_UnstampedClaimStillFallsBack|TestResolveContinuationForScope_ScopeOrderIsDeterministic")
	if !ok {
		t.Errorf("lane-scope continuation resolution is missing, or it no longer tries the "+
			"inbox claim first (G1/PR #363 regression):\n%s", out)
	}
}

// TestC1104_004_ResolveNeverInventsOrLeaksABinding — NEGATIVE. Unknown/empty/
// nil/blank scope lists, an entry with no snapshot ref, and a corrupt registry
// must ALL resolve nil rather than hand one lane another lane's work or crash
// the orchestrator mid-cycle. Also pins that the original claim-only
// ResolveContinuation still ignores the registry entirely, so callers wanting
// G1 semantics are byte-identical after the extension.
func TestC1104_004_ResolveNeverInventsOrLeaksABinding(t *testing.T) {
	ok, out := runGoTest(t, inboxmoverPkg,
		"TestResolveContinuationForScope_NoBindingIsNil|TestResolveContinuationForScope_EmptySnapshotIsNotABinding|TestResolveContinuationForScope_CorruptRegistryIsNilNotPanic|TestResolveContinuation_ClaimOnlyPathUnchanged")
	if !ok {
		t.Errorf("scope resolution invents a binding for an unrelated/blank scope, accepts an "+
			"empty snapshot ref, panics on a corrupt registry, or leaked the fallback into the "+
			"claim-only entry point:\n%s", out)
	}
}

// TestC1104_005_FailReleaseRegistersLaneScopeBindingOnTheCleanGate — AC1, the
// produce side. Drives the REAL stampContinuationManifest over a real git
// fixture: a FAIL whose scope exists only as a lane-scope pin registers a
// binding mirroring the manifest (every todo id of a multi-id lane), the
// registration rides the EXISTING carry-forward Clean gate (conflicting work
// registers nothing, since re-adopting it would only re-reject it every cycle),
// and a cycle with no lane-scope pin registers nothing at all — the extension
// is additive, never a blanket "always register" that would mint phantom keys.
func TestC1104_005_FailReleaseRegistersLaneScopeBindingOnTheCleanGate(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestStampContinuationManifest_RegistersLaneScopeBinding|TestStampContinuationManifest_RegistersEveryLaneScopeID|TestStampContinuationManifest_NoLaneScopeRegistersNothing|TestStampContinuationManifest_UnstampableWorkRegistersNothing")
	if !ok {
		t.Errorf("the preserve decision does not register a lane-scope binding, registers work "+
			"the carry-forward screen rejected (unresumable binding), or mints a binding for a "+
			"cycle that has no lane scope:\n%s", out)
	}
}

// TestC1104_006_NonClaimCycleAdoptsPreservedWorkEndToEnd — the composed proof,
// and the anti-no-op predicate: RunCycle through the PRODUCTION resolver
// closure. A later cycle scoped to the same lane-scope todo id — with no inbox
// item and no processing claim anywhere in the repo — re-seeds its worktree
// from the preserved snapshot and serves the prior findings to build; a cycle
// scoped to a DIFFERENT id must not inherit that work (cross-lane
// contamination); and the orchestrator must actually hand the resolver this
// cycle's pinned lane-scope ids, without which no fallback is implementable.
func TestC1104_006_NonClaimCycleAdoptsPreservedWorkEndToEnd(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestRunCycle_AdoptsContinuationFromLaneScopeWithoutAnyClaim|TestRunCycle_UnrelatedLaneScopeDoesNotAdopt|TestAdoptContinuation_PassesLaneScopeIDsToResolver")
	if !ok {
		t.Errorf("a non-claim lane still cannot adopt its own preserved work (the cycle-1078 "+
			"orphan class is open), or it adopts another lane's:\n%s", out)
	}
}

// TestC1104_007_ExistingContinuationContractStillGreen — AC4 anti-regression.
// The G1 claim path (snapshot, manifest gating, adopt-time re-screen,
// release-time stamping, quarantine shedding) must still hold after the
// extension. This is the predicate that fails if Builder "adds" the lane-scope
// class by weakening or rerouting the landed claim behaviour.
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
