//go:build acs

package cycle1291

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

	taskSlug = "fix-contract-block-identity-subset-collapse"
)

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
		}
	}
}

func TestC1291_001_subset_repair_still_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_SubsetRepairStillEscalates$",
		[]string{"TestContractCorrection_SubsetRepairStillEscalates"})
}

func TestC1291_002_superset_regression_still_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_SupersetRegressionStillEscalates$",
		[]string{"TestContractCorrection_SupersetRegressionStillEscalates"})
}

func TestC1291_003_disjoint_violation_sets_do_not_escalate(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_DisjointViolationSetsDoNotEscalate$",
		[]string{"TestContractCorrection_DisjointViolationSetsDoNotEscalate"})
}

func TestC1291_004_reordered_violation_set_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_ReorderedViolationSetEscalates$",
		[]string{"TestContractCorrection_ReorderedViolationSetEscalates"})
}

func TestC1291_005_uncoded_reasons_fall_back_to_text_identity(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_UncodedReasonsFallBackToTextIdentity$",
		[]string{"TestContractCorrection_UncodedReasonsFallBackToTextIdentity"})
}

func TestC1291_006_cycle1289_identity_axes_preserved(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestContractCorrection_(DifferingBlockReasonsDoNotEscalate|NormalizedIdenticalReasonsEscalate|HotBreakerEscalatesOnFirstCorrection)$",
		[]string{
			"TestContractCorrection_DifferingBlockReasonsDoNotEscalate",
			"TestContractCorrection_NormalizedIdenticalReasonsEscalate",
			"TestContractCorrection_HotBreakerEscalatesOnFirstCorrection",
		})
}

func TestC1291_007_escalation_feature_set_intact(t *testing.T) {
	runGoTest(t, corePkg,
		"^Test(ContractCorrection_|ContractEscalation_|FormatContractGateDemotionWarn|ChainReviewers_|UniversalContractFallbackMatchesLLMRouteDefault)",
		[]string{
			"TestContractCorrection_SecondBlockEscalatesToProfileFallback",
			"TestContractCorrection_FirstBlockDoesNotEscalate",
			"TestContractCorrection_NonContractRejectionNeverEscalates",
			"TestContractCorrection_NoDeclaredFallbackEscalatesToUniversalClaude",
			"TestContractCorrection_SameFamilyFallbackIsNotAnEscalation",
			"TestContractCorrection_CompliantFallbackPreventsCircuitOpen",
			"TestContractCorrection_CircuitOpenWarnsAndFilesItem",
			"TestContractEscalation_RespectsAllowedCLIs",
			"TestUniversalContractFallbackMatchesLLMRouteDefault",
		})
}

func TestC1291_008_core_and_deliverable_build_and_vet(t *testing.T) {
	for _, pkg := range []string{corePkg, deliverablePkg} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", pkg)
		if code != 0 || err != nil {
			t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", pkg, code, err, stdout, stderr)
		}
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", corePkg, deliverablePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC1291_009_eval_file_passes_quality_check(t *testing.T) {
	root := acsassert.RepoRoot(t)
	evalPath := filepath.Join(root, ".evolve", "evals", taskSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval %s command %q classified level %d: %s", taskSlug, c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", taskSlug, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", taskSlug, len(res.Commands))
	}
}
