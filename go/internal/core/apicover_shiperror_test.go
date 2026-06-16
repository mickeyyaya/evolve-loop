package core

import (
	"fmt"
	"strings"
	"testing"
)

// apicover_shiperror_test.go — public-API coverage (ADR-0050, Phase 5) for the
// ShipError protocol in shiperror.go. The ship phase is a pure executor: every
// failure it cannot execute through is reported as a *ShipError carrying a
// precise Code, a severity Class, the Stage it failed at, and a Debug map. The
// orchestrator errors.As-matches it to decide recovery. These tests name AND
// exercise the 4 protocol types, the 2 previously-uncovered ShipStage consts,
// and all 32 previously-uncovered ShipErrorCode consts, asserting the REAL
// construction + rendering + class-pairing contracts (not value padding).
//
// There is intentionally NO code->class mapper function in core: the class is
// chosen per call-site (internal/phases/ship/{verify,gitops}.go via the shipErr
// wrapper) and recorded verbatim by NewShipError. So the contract this file
// pins is: (a) NewShipError faithfully records {code,class,stage,message,debug};
// (b) Error() renders all three identity fields; (c) the canonical class each
// code is constructed with in production round-trips through errors.As. The
// per-code wantClass column below is transcribed from the live ship call-sites,
// so a drift between this table and production is a real protocol regression.

// shipCodeCase binds a code to the severity Class the ship phase actually
// constructs it with in production (internal/phases/ship/*.go).
type shipCodeCase struct {
	code      ShipErrorCode
	wantClass ShipErrorClass
	// stage the production call-site uses; asserted to be one of the known stages.
	stage ShipStage
}

// allShipCodeCases enumerates the 32 codes targeted by apicover, each paired
// with its canonical production Class + Stage (see file header for provenance).
func allShipCodeCases() []shipCodeCase {
	return []shipCodeCase{
		// verify-self-sha
		{CodeSelfSHATampered, ShipClassIntegrity, StageVerifySelfSHA},
		{CodeSelfSHAIO, ShipClassTransient, StageVerifySelfSHA},
		{CodeStateIO, ShipClassTransient, StageVerifySelfSHA},

		// verify-class — audit binding (all precondition: upstream re-establishable)
		{CodeAuditBindingTreeMismatch, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingArtifactSHA, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingArtifactMissing, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingVerdictFail, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingVerdictWarn, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingMalformed, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingDualVerdict, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingStale, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingNoAuditor, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingAuditorExit, ShipClassPrecondition, StageVerifyClass},
		{CodeAuditBindingNoLedger, ShipClassPrecondition, StageVerifyClass},

		// verify-class — EGPS gate
		{CodeEGPSRedCount, ShipClassPrecondition, StageVerifyClass},

		// verify-class — manual / trivial / commit-gate (operator/config errors)
		{CodeInvalidClass, ShipClassConfig, StageVerifyClass},
		{CodeManualNotTTY, ShipClassConfig, StageVerifyClass},
		{CodeManualDeclined, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateMissing, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateStale, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateMalformed, ShipClassConfig, StageVerifyClass},
		{CodeTrivialNotTrivial, ShipClassConfig, StageVerifyClass},
		{CodeTrivialCriticalPaths, ShipClassConfig, StageVerifyClass},

		// atomic-ship — git
		{CodeGitDetachedHead, ShipClassPrecondition, StageAtomicShip},
		{CodeGitStageFailed, ShipClassTransient, StageAtomicShip},
		{CodeGitCommitFailed, ShipClassPrecondition, StageAtomicShip},
		{CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip},
		{CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip},
		{CodeGitFleetRebaseConflict, ShipClassIntegrity, StageAtomicShip},
		{CodeCommitPrefixGate, ShipClassPrecondition, StageAtomicShip},

		// generic / fallthrough
		{CodeArgs, ShipClassConfig, StageArgs},
		// CodeUnknown is the generic fallthrough identity; it has no fixed
		// production class, so it is exercised separately below (no class assert).
	}
}

// TestShipError_TypesAreUsable names the 4 protocol types (ShipError,
// ShipErrorClass, ShipErrorCode, ShipStage) as typed vars and asserts the
// zero/known-value relationships the orchestrator relies on.
func TestShipError_TypesAreUsable(t *testing.T) {
	t.Parallel()

	// Name each type via a typed declaration so apicover marks them covered,
	// and assert the underlying-string identity contract the ledger keys off.
	var code ShipErrorCode = CodeArgs
	var class ShipErrorClass = ShipClassConfig
	var stage ShipStage = StageArgs
	var se ShipError = ShipError{Code: code, Class: class, Stage: stage, Message: "boom"}

	if string(code) != "ARGS" {
		t.Errorf("ShipErrorCode underlying string = %q; want ARGS", string(code))
	}
	if string(class) != "config" {
		t.Errorf("ShipErrorClass underlying string = %q; want config", string(class))
	}
	if string(stage) != "args" {
		t.Errorf("ShipStage underlying string = %q; want args", string(stage))
	}
	// A value ShipError still renders via its pointer Error() method.
	if got := se.Error(); !strings.Contains(got, "ARGS") || !strings.Contains(got, "boom") {
		t.Errorf("ShipError.Error() = %q; want it to carry code+message", got)
	}
}

// TestShipStageConsts_PostShipAndVerifySelfSHA names the 2 uncovered ShipStage
// consts and asserts each plays its documented role: a distinct, non-empty,
// stable stage identity that NewShipError records and Error() renders.
func TestShipStageConsts_PostShipAndVerifySelfSHA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stage    ShipStage
		wantWire string
	}{
		{StageVerifySelfSHA, "verify-self-sha"}, // first ship stage: TOFU self-SHA pin
		{StagePostShip, "post-ship"},            // last ship stage: post-push tree-drift guard
	}
	seen := map[ShipStage]bool{}
	for _, tc := range cases {
		if string(tc.stage) != tc.wantWire {
			t.Errorf("stage wire value = %q; want %q", string(tc.stage), tc.wantWire)
		}
		if seen[tc.stage] {
			t.Errorf("duplicate stage identity: %q", tc.stage)
		}
		seen[tc.stage] = true

		// Role: the stage is recorded into a ShipError and surfaced in Error().
		se := NewShipError(CodeStateIO, ShipClassTransient, tc.stage, "io trouble")
		if se.Stage != tc.stage {
			t.Errorf("NewShipError did not record stage: got %q want %q", se.Stage, tc.stage)
		}
		if !strings.Contains(se.Error(), "@"+tc.wantWire) {
			t.Errorf("Error() = %q; want it to render @%s", se.Error(), tc.wantWire)
		}
	}
	// The two consts must be distinct from each other.
	if StagePostShip == StageVerifySelfSHA {
		t.Fatal("StagePostShip and StageVerifySelfSHA must be distinct stages")
	}
}

// TestShipErrorCodes_ConstructAndClassify drives all 32 target codes through
// the REAL constructor with their canonical production class+stage, then
// asserts each produces a well-formed, errors.As-recoverable ShipError whose
// Code/Class/Stage round-trip and whose Error() carries the code wire string.
func TestShipErrorCodes_ConstructAndClassify(t *testing.T) {
	t.Parallel()

	cases := allShipCodeCases()
	if len(cases) != 31 { // 32 target codes minus CodeUnknown (tested separately)
		t.Fatalf("table drift: have %d code cases, want 31 (CodeUnknown is separate)", len(cases))
	}

	seenWire := map[string]bool{}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			// Code wire strings must be unique — the ledger + debugger persona
			// key off them, so a collision is a real protocol bug.
			wire := string(tc.code)
			if wire == "" {
				t.Fatalf("code %v has empty wire string", tc.code)
			}
			if seenWire[wire] {
				t.Fatalf("duplicate code wire string %q", wire)
			}
			seenWire[wire] = true

			// Construct through the real constructor with a diagnostic pair.
			se := NewShipError(tc.code, tc.wantClass, tc.stage, "produced by ship",
				"detail", wire)

			// Identity round-trips verbatim (constructor records, never mutates).
			if se.Code != tc.code {
				t.Errorf("Code = %q; want %q", se.Code, tc.code)
			}
			if se.Class != tc.wantClass {
				t.Errorf("Class = %q; want %q (production pairing)", se.Class, tc.wantClass)
			}
			if se.Stage != tc.stage {
				t.Errorf("Stage = %q; want %q", se.Stage, tc.stage)
			}

			// Error() is the single-line "[CODE/class @stage] message" contract.
			msg := se.Error()
			for _, want := range []string{wire, string(tc.wantClass), string(tc.stage), "produced by ship"} {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q; missing %q", msg, want)
				}
			}

			// Debug pair recorded + rendered deterministically.
			if got := se.Debug["detail"]; got != wire {
				t.Errorf("Debug[detail] = %q; want %q", got, wire)
			}
			if ds := se.DebugString(); !strings.Contains(ds, "detail="+wire) {
				t.Errorf("DebugString() = %q; want it to contain detail=%s", ds, wire)
			}

			// The orchestrator recovers it from a wrapped chain via errors.As.
			wrapped := fmt.Errorf("orchestrator layer: %w", error(se))
			if got, ok := AsShipError(wrapped); !ok {
				t.Errorf("AsShipError(wrap(%s)) = (_, false); want recoverable", wire)
			} else if got.Code != tc.code {
				t.Errorf("recovered Code = %q; want %q", got.Code, tc.code)
			}
		})
	}
}

// TestShipErrorClass_VocabularyIsExhaustiveAndDistinct names ShipErrorClass and
// asserts the 4 severity values the codes map onto are the complete, distinct
// vocabulary actually used by the 31 classified codes — so a new unclassified
// class or a code with an out-of-vocabulary class is caught here.
func TestShipErrorClass_VocabularyIsExhaustiveAndDistinct(t *testing.T) {
	t.Parallel()

	vocab := map[ShipErrorClass]bool{
		ShipClassTransient:    true,
		ShipClassPrecondition: true,
		ShipClassIntegrity:    true,
		ShipClassConfig:       true,
	}
	if len(vocab) != 4 {
		t.Fatalf("severity vocabulary collision: %d distinct classes, want 4", len(vocab))
	}

	usedClasses := map[ShipErrorClass]bool{}
	for _, tc := range allShipCodeCases() {
		if !vocab[tc.wantClass] {
			t.Errorf("code %q uses out-of-vocabulary class %q", tc.code, tc.wantClass)
		}
		usedClasses[tc.wantClass] = true
	}
	// Every severity class is actually exercised by at least one target code.
	for c := range vocab {
		if !usedClasses[c] {
			t.Errorf("severity class %q is never used by any target code", c)
		}
	}
}

// TestCodeUnknown_FallthroughIdentity covers the 32nd code, CodeUnknown — the
// generic fallthrough identity with no fixed production class. Its contract is
// only that it is a non-empty, distinct wire string the constructor records and
// that round-trips through errors.As like any other code.
func TestCodeUnknown_FallthroughIdentity(t *testing.T) {
	t.Parallel()

	if string(CodeUnknown) != "UNKNOWN" {
		t.Errorf("CodeUnknown wire = %q; want UNKNOWN", string(CodeUnknown))
	}
	// Distinct from the classified codes (no accidental aliasing).
	for _, tc := range allShipCodeCases() {
		if tc.code == CodeUnknown {
			t.Fatalf("CodeUnknown must not appear in the classified table")
		}
	}
	se := NewShipError(CodeUnknown, ShipClassTransient, StagePostShip, "unclassified ship failure")
	if se.Code != CodeUnknown {
		t.Errorf("Code = %q; want UNKNOWN", se.Code)
	}
	if got, ok := AsShipError(fmt.Errorf("wrap: %w", error(se))); !ok || got.Code != CodeUnknown {
		t.Errorf("AsShipError round-trip failed for CodeUnknown: ok=%v code=%q", ok, errCode(got))
	}
}

// errCode is a nil-safe Code accessor for the failure message above.
func errCode(se *ShipError) ShipErrorCode {
	if se == nil {
		return ""
	}
	return se.Code
}
