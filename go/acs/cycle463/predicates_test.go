//go:build acs

// Package cycle463 materialises the cycle-463 acceptance criteria for the
// three triage-committed tasks (## top_n only; T2 advisor-plan-measured-
// usage-inputs was deferred by triage and gets ZERO predicates per R9.3):
//
//	T1 advisor-plan-prompt-tier-elicitation      (P1) → C463_001..006
//	T3 runner-overlay-observability-dossier-model-source (P3) → C463_007..012
//	T4 routing-replay-clamp-golden-matrix        (P4) → C463_013..018
//
// Each predicate shells to the NAMED RED unit test(s) written this cycle in
// go/internal/{core,router,phases/runner,dossier} (requireTestsRan guards
// against a silent "no tests to run" false-green), so a predicate can never
// pass on unwritten or renamed work. 1:1 AC-materialization: 18 predicates +
// 0 manual + 0 removed = 18 ACs (6 per task, matching each task's eval file
// in .evolve/evals/), none double-counted.
//
// RED strategy (verified in test-report.md "RED Run Output"): every
// predicate below currently FAILs — either the target package fails to
// compile (phasespec.PhaseSpec/router.Clamp/phasetiming.Entry/runner.Options
// carry no new fields yet; router.RejectionsFromClamps does not exist) or
// the named test asserts a behavior the production code does not implement
// yet (fast-below-min clamps to empty, not up to balanced). The "regression"
// predicates (C463_006/012/018) fail for the same reason: a package that
// doesn't compile cannot pass `go vet`/`go test` for the whole package.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C463_004 (absent cli/tier fields must never fabricate an
//	            overlay), C463_009 (pin must win over a non-empty overlay),
//	            C463_016 (no clamp relaxation for a persuasive justification)
//	Edge/OOD:   C463_005 (tier vocabulary confinement: high/top/raw model),
//	            C463_011 (legacy workspace with no model metadata),
//	            C463_017 (legacy cycle-459-shape response byte-identical)
//	Semantic:   C463_002 vs C463_014 (guardrail PROJECTION is distinct from
//	            guardrail RENDERING — both must hold), C463_013 vs C463_017
//	            (an in-bounds proposal overlays; a legacy one never does —
//	            only jointly satisfiable by correct gating on cli/tier
//	            presence)
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

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
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

// ---- T1 advisor-plan-prompt-tier-elicitation (P1) ----

// TestC463_001_PersonaPathElicitsTierAndCLI (AC1): the persona-path plan
// prompt shows the {cli,tier} schema/example on an existing phase plus the
// operator's fast-mechanical/deep-judgment model policy. RED today:
// composePlanPrompt never renders either.
func TestC463_001_PersonaPathElicitsTierAndCLI(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_ElicitsTierAndCLI|TestComposePlanPrompt_RendersOperatorModelPolicy", true, corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("persona-path tier/cli elicitation contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_002_PhaseCardsProjectDispatchGuardrails (AC2): phaseCardsFromCatalog
// projects a spec's allowed_clis/model_tier_envelope onto PhaseCard and the
// composed prompt renders the resulting guardrail lines. RED today:
// phasespec.PhaseSpec carries neither field (compile failure).
func TestC463_002_PhaseCardsProjectDispatchGuardrails(t *testing.T) {
	out, code := runGoTest(t, "TestPhaseCardsFromCatalog_ProjectsDispatchGuardrails|TestComposePlanPrompt_RendersGuardrailLinesForCatalogPhase", true, corePkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("dispatch-guardrail projection contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_003_QuotaWalledCLINamedUnavailable (AC3): a fully-exhausted
// benched CLI is named WALLED/unavailable in the composed prompt. RED today:
// the rendered sentence says only "benched (<reason>) until <time>".
func TestC463_003_QuotaWalledCLINamedUnavailable(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_NamesBenchedCLIAsWalled", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("quota-wall wording contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_004_AbsentFieldsDegradeByteIdentical (AC4, negative): a legacy
// advisor response without cli/tier keys parses to empty CLI/Tier. Expected
// pre-existing GREEN once the package compiles (parsePhasePlan already
// zero-values unset fields) — this predicate still gates it as part of the
// T1 contract so a future change cannot silently regress it.
func TestC463_004_AbsentFieldsDegradeByteIdentical(t *testing.T) {
	out, code := runGoTest(t, "TestParsePhasePlan_AbsentCLITierFieldsStayEmpty", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("absent-field degrade-path contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_005_TierVocabularyConfinement (AC5, edge): high/top/raw-model-name
// proposals never survive sanitizeAdvisorTier. Expected pre-existing GREEN
// once the package compiles (sanitizeAdvisorTier already confines to
// fast/balanced/deep).
func TestC463_005_TierVocabularyConfinement(t *testing.T) {
	out, code := runGoTest(t, "TestSanitizeAdvisorTier_RejectsHighTopAndRawModel", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("tier-vocabulary confinement contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_006_T1RegressionVetAndRace (AC6, regression): internal/core and
// internal/router must be vet-clean and fully -race green — including every
// pre-existing pin. RED today while the T1 tests above are red (a
// non-compiling package fails go vet too).
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

// ---- T3 runner-overlay-observability-dossier-model-source (P3) ----

// TestC463_007_OverlayAppliedLogLine (AC1): the runner logs a grep-able line
// naming the phase, overlay cli, and overlay tier when the MR4c overlay
// applies. RED today: runner.Options carries no injectable diag logger
// (compile failure), and nothing is logged at the seam either way.
func TestC463_007_OverlayAppliedLogLine(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_AdvisorOverlayAppliedLogsLine", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("overlay-applied log contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_008_NoOverlayLogLine (AC2): the runner logs an explicit
// "no advisor overlay (profile default)" line when overlay fields are empty.
func TestC463_008_NoOverlayLogLine(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_NoAdvisorOverlayLogsProfileDefault", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-overlay log contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_009_PinWinsSourceIsPin (AC3, negative): a policy pin wins over a
// non-empty advisor overlay, and the recorded model source is "pin", never
// "advisor". RED today: core.PhaseResponse carries no ModelSource field.
func TestC463_009_PinWinsSourceIsPin(t *testing.T) {
	out, code := runGoTest(t, "TestRunner_PolicyPinWinsOverAdvisorOverlay_SourceIsPin", true, runnerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("pin-wins model-source contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_010_DossierCarriesModelSourceAndResolvedModel (AC4): the dossier
// (via phase-timing.json ingestion) carries per-phase model_source and
// resolved_model. RED today: phasetiming.Entry/dossier.PhaseRecord carry
// neither field, and runner never populates PhaseResponse.ModelSource.
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

// TestC463_011_LegacyWorkspaceDegradesSafely (AC5, edge): a legacy
// phase-timing.json without model-source metadata builds a valid dossier
// with the field absent, never fabricated, never an error.
func TestC463_011_LegacyWorkspaceDegradesSafely(t *testing.T) {
	out, code := runGoTest(t, "TestBuild_LegacyWorkspaceWithoutModelMetadataDegradesSafely", true, dossierPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("legacy-workspace safe-degrade contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_012_T3RegressionVetAndRace (AC6, regression): internal/phases/
// runner, internal/dossier, and internal/core must be vet-clean and fully
// -race green.
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

// ---- T4 routing-replay-clamp-golden-matrix (P4) ----

// TestC463_013_ReplayOverlayAppliesUnderAuto (AC1): a recorded plan response
// carrying {cli,tier} drives floor-clamp -> model-routing clamp -> soft
// dispatch overlay to the clamped cli/tier, end to end.
func TestC463_013_ReplayOverlayAppliesUnderAuto(t *testing.T) {
	out, code := runGoTest(t, "TestReplayPlanFromResponse_ModelRoutingOverlayAppliesUnderAuto", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("e2e replay-overlay contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_014_ClampMatrixAndRejectionMapping (AC2): the 4-case clamp matrix
// (out-of-envelope, fast-below-min clamps UP, disallowed CLI, catalog-miss)
// each fires exactly one Phase-attributed clamp, and RejectionsFromClamps
// maps clamps onto the PlanRejection shape. RED today: Clamp carries no
// Phase field and RejectionsFromClamps does not exist (compile failure);
// the fast-below-min case clamps to empty today, not up to balanced.
func TestC463_014_ClampMatrixAndRejectionMapping(t *testing.T) {
	out, code := runGoTest(t, "TestClampPlanModelRouting_Matrix|TestRejectionsFromClamps_NamesPhaseAndReason", true, routerPkg)
	requireTestsRan(t, out, 2)
	if code != 0 {
		t.Errorf("clamp-matrix + rejection-mapping contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_015_LiveShapePromptReplay (AC3): the composed plan prompt,
// rendered against a recorded RouteInput fixture, carries the {cli,tier}
// schema AND per-phase guardrail lines together.
func TestC463_015_LiveShapePromptReplay(t *testing.T) {
	out, code := runGoTest(t, "TestComposePlanPrompt_LiveShapeReplayCarriesTierSchemaAndGuardrails", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("live-shape prompt replay contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_016_NoClampRelaxation (AC4, negative): a persuasive Justification
// string never widens the envelope or skips the guardrail.
func TestC463_016_NoClampRelaxation(t *testing.T) {
	out, code := runGoTest(t, "TestClampPlanModelRouting_NoRelaxationEvenWithJustification", true, routerPkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("no-clamp-relaxation contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_017_LegacyReplayByteIdentical (AC5, edge): replaying the exact
// legacy (cycle-459-shape) response yields zero model-routing clamps and a
// dispatch byte-identical to the profile-static baseline.
func TestC463_017_LegacyReplayByteIdentical(t *testing.T) {
	out, code := runGoTest(t, "TestReplayPlanFromResponse_LegacyResponseByteIdenticalDispatch", true, corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("legacy-replay byte-identical contract is red (exit=%d)\n%s", code, out)
	}
}

// TestC463_018_T4RegressionVetAndRace (AC6, regression): internal/router and
// internal/core must be vet-clean and fully -race green.
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
