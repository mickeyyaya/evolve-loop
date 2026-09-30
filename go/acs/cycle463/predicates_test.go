//go:build acs

package cycle463

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	routerPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/router"
	runnerPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	dossierPkg = "github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (out string, code int) {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout + "\n" + stderr, code
}

func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

func TestC463_001_PersonaPathElicitsTierAndCLI(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_ElicitsTierAndCLI|TestComposePlanPrompt_RendersOperatorModelPolicy", true, corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("persona-path tier/cli elicitation contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_002_PhaseCardsProjectDispatchGuardrails(t *testing.T) {
	out, code := runGoTest(t, "TestPhaseCardsFromCatalog_ProjectsDispatchGuardrails|TestComposePlanPrompt_RendersGuardrailLinesForCatalogPhase", true, corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("dispatch-guardrail projection contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_003_QuotaWalledCLINamedUnavailable(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_NamesBenchedCLIAsWalled", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("quota-wall wording contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_004_AbsentFieldsDegradeByteIdentical(t *testing.T) {
	out, code := runGoTest(t, "TestParsePhasePlan_AbsentCLITierFieldsStayEmpty", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("absent-field degrade-path contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_005_TierVocabularyConfinement(t *testing.T) {
	out, code := runGoTest(t, "TestSanitizeAdvisorTier_RejectsHighTopAndRawModel", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("tier-vocabulary confinement contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_006_T1RegressionVetAndRace(t *testing.T) {
	for _, pkg := range []string{corePkg, routerPkg} {
		if stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "vet", pkg); code != 0 {
			t.Errorf("go vet %s exit=%d\n%s%s", pkg, code, stdout, stderr)
		}
	}
	out, code := runGoTest(t, "", true, corePkg, routerPkg)
	if code != 0 {
		t.Errorf("T1 package -race suite exit=%d\n%s", code, out)
	}
}

func TestC463_007_OverlayAppliedLogLine(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_AdvisorOverlayAppliedLogsLine", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("overlay-applied log contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_008_NoOverlayLogLine(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_NoAdvisorOverlayLogsProfileDefault", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-overlay log contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_009_PinWinsSourceIsPin(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_PolicyPinWinsOverAdvisorOverlay_SourceIsPin", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("pin-wins model-source contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_010_DossierCarriesModelSourceAndResolvedModel(t *testing.T) {
	out1, code1 := runGoTest(t, "TestBuild_ProjectsModelSourceAndResolvedModel", true, dossierPkg)
	requireTestsRan(t, out1, 1)
	out2, code2 := runGoTest(t, "TestRun_ModelSourceReflectsResolutionPath", true, runnerPkg)
	requireTestsRan(t, out2, 1)
	if code1 != 0 {
		t.Errorf("dossier model-source contract is red (exit=%d)\n%s", code1, out1)
	}
	if code2 != 0 {
		t.Errorf("runner model-source resolution contract is red (exit=%d)\n%s", code2, out2)
	}
}

func TestC463_011_LegacyWorkspaceDegradesSafely(t *testing.T) {
	out, code := runGoTest(t, "TestBuild_LegacyWorkspaceWithoutModelMetadataDegradesSafely", true, dossierPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("legacy-workspace safe-degrade contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_012_T3RegressionVetAndRace(t *testing.T) {
	for _, pkg := range []string{runnerPkg, dossierPkg, corePkg} {
		if stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "vet", pkg); code != 0 {
			t.Errorf("go vet %s exit=%d\n%s%s", pkg, code, stdout, stderr)
		}
	}
	out, code := runGoTest(t, "", true, runnerPkg, dossierPkg, corePkg)
	if code != 0 {
		t.Errorf("T3 package -race suite exit=%d\n%s", code, out)
	}
}

func TestC463_013_ReplayOverlayAppliesUnderAuto(t *testing.T) {
	out, code := runGoTest(t, "TestReplayPlanFromResponse_ModelRoutingOverlayAppliesUnderAuto", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("e2e replay-overlay contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_014_ClampMatrixAndRejectionMapping(t *testing.T) {
	out, code := runGoTest(t, "TestClampPlanModelRouting_Matrix|TestRejectionsFromClamps_NamesPhaseAndReason", true, routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("clamp-matrix + rejection-mapping contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_015_LiveShapePromptReplay(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_LiveShapeReplayCarriesTierSchemaAndGuardrails", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("live-shape prompt replay contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_016_NoClampRelaxation(t *testing.T) {
	out, code := runGoTest(t, "TestClampPlanModelRouting_NoRelaxationEvenWithJustification", true, routerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-clamp-relaxation contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_017_LegacyReplayByteIdentical(t *testing.T) {
	out, code := runGoTest(t, "TestReplayPlanFromResponse_LegacyResponseByteIdenticalDispatch", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("legacy-replay byte-identical contract is red (exit=%d)\n%s", code, out)
	}
}

func TestC463_018_T4RegressionVetAndRace(t *testing.T) {
	for _, pkg := range []string{routerPkg, corePkg} {
		if stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "vet", pkg); code != 0 {
			t.Errorf("go vet %s exit=%d\n%s%s", pkg, code, stdout, stderr)
		}
	}
	out, code := runGoTest(t, "", true, routerPkg, corePkg)
	if code != 0 {
		t.Errorf("T4 package -race suite exit=%d\n%s", code, out)
	}
}
