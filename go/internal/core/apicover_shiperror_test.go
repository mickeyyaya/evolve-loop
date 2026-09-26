package core

import (
	"fmt"
	"strings"
	"testing"
)

type shipCodeCase struct {
	code      ShipErrorCode
	wantClass ShipErrorClass
	stage     ShipStage
}

// allShipCodeCases pairs each code with the Class and Stage its production call site in internal/phases/ship uses.
func allShipCodeCases() []shipCodeCase {
	return []shipCodeCase{
		{CodeExplanationDocumentation, ShipClassPrecondition, StageVerifyExplanation},

		{CodeSelfSHATampered, ShipClassIntegrity, StageVerifySelfSHA},
		{CodeSelfSHAIO, ShipClassTransient, StageVerifySelfSHA},
		{CodeStateIO, ShipClassTransient, StageVerifySelfSHA},

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

		{CodeEGPSRedCount, ShipClassPrecondition, StageVerifyClass},

		{CodeControlPlaneViolation, ShipClassPrecondition, StageVerifyClass},

		{CodeInvalidClass, ShipClassConfig, StageVerifyClass},
		{CodeManualNotTTY, ShipClassConfig, StageVerifyClass},
		{CodeManualDeclined, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateMissing, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateStale, ShipClassConfig, StageVerifyClass},
		{CodeCommitGateMalformed, ShipClassConfig, StageVerifyClass},
		{CodeTrivialNotTrivial, ShipClassConfig, StageVerifyClass},
		{CodeTrivialCriticalPaths, ShipClassConfig, StageVerifyClass},

		{CodeGitDetachedHead, ShipClassPrecondition, StageAtomicShip},
		{CodeGitStageFailed, ShipClassTransient, StageAtomicShip},
		{CodeGitCommitFailed, ShipClassPrecondition, StageAtomicShip},
		{CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip},
		{CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip},
		{CodeGitFleetRebaseConflict, ShipClassIntegrity, StageAtomicShip},
		{CodeCommitPrefixGate, ShipClassPrecondition, StageAtomicShip},

		{CodeArgs, ShipClassConfig, StageArgs},
		// CodeUnknown has no fixed production class, so it is exercised separately.
	}
}

func TestShipError_TypesAreUsable(t *testing.T) {
	t.Parallel()

	// Typed declarations name each type for apicover.
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
	if got := se.Error(); !strings.Contains(got, "ARGS") || !strings.Contains(got, "boom") {
		t.Errorf("ShipError.Error() = %q; want it to carry code+message", got)
	}
}

func TestShipStageConsts_PostShipAndVerifySelfSHA(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stage    ShipStage
		wantWire string
	}{
		{StageVerifyExplanation, "verify-explanation"},
		{StageVerifySelfSHA, "verify-self-sha"},
		{StagePostShip, "post-ship"},
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

		se := NewShipError(CodeStateIO, ShipClassTransient, tc.stage, "io trouble")
		if se.Stage != tc.stage {
			t.Errorf("NewShipError did not record stage: got %q want %q", se.Stage, tc.stage)
		}
		if !strings.Contains(se.Error(), "@"+tc.wantWire) {
			t.Errorf("Error() = %q; want it to render @%s", se.Error(), tc.wantWire)
		}
	}
	if StagePostShip == StageVerifySelfSHA || StageVerifyExplanation == StageVerifySelfSHA {
		t.Fatal("ship stages must have distinct identities")
	}
}

func TestShipErrorCodes_ConstructAndClassify(t *testing.T) {
	t.Parallel()

	cases := allShipCodeCases()
	if len(cases) != 33 {
		t.Fatalf("table drift: have %d code cases, want 33 (CodeUnknown is separate)", len(cases))
	}

	seenWire := map[string]bool{}
	for _, tc := range cases {
		t.Run(string(tc.code), func(t *testing.T) {
			wire := string(tc.code)
			if wire == "" {
				t.Fatalf("code %v has empty wire string", tc.code)
			}
			if seenWire[wire] {
				t.Fatalf("duplicate code wire string %q", wire)
			}
			seenWire[wire] = true

			se := NewShipError(tc.code, tc.wantClass, tc.stage, "produced by ship",
				"detail", wire)

			if se.Code != tc.code {
				t.Errorf("Code = %q; want %q", se.Code, tc.code)
			}
			if se.Class != tc.wantClass {
				t.Errorf("Class = %q; want %q (production pairing)", se.Class, tc.wantClass)
			}
			if se.Stage != tc.stage {
				t.Errorf("Stage = %q; want %q", se.Stage, tc.stage)
			}

			msg := se.Error()
			for _, want := range []string{wire, string(tc.wantClass), string(tc.stage), "produced by ship"} {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q; missing %q", msg, want)
				}
			}

			if got := se.Debug["detail"]; got != wire {
				t.Errorf("Debug[detail] = %q; want %q", got, wire)
			}
			if ds := se.DebugString(); !strings.Contains(ds, "detail="+wire) {
				t.Errorf("DebugString() = %q; want it to contain detail=%s", ds, wire)
			}

			wrapped := fmt.Errorf("orchestrator layer: %w", error(se))
			if got, ok := AsShipError(wrapped); !ok {
				t.Errorf("AsShipError(wrap(%s)) = (_, false); want recoverable", wire)
			} else if got.Code != tc.code {
				t.Errorf("recovered Code = %q; want %q", got.Code, tc.code)
			}
		})
	}
}

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
	for c := range vocab {
		if !usedClasses[c] {
			t.Errorf("severity class %q is never used by any target code", c)
		}
	}
}

func TestCodeUnknown_FallthroughIdentity(t *testing.T) {
	t.Parallel()

	if string(CodeUnknown) != "UNKNOWN" {
		t.Errorf("CodeUnknown wire = %q; want UNKNOWN", string(CodeUnknown))
	}
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

func errCode(se *ShipError) ShipErrorCode {
	if se == nil {
		return ""
	}
	return se.Code
}
