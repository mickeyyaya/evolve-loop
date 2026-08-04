//go:build acs

// Package cycle1291 materializes the cycle-1291 acceptance criteria for the one
// fleet-scoped todo pinned to this lane (contract-block-cli-escalation), which
// triage resolved to a single top_n task:
//
//	fix-contract-block-identity-subset-collapse
//
// WHAT THIS CYCLE CLOSES. cycle-1289 landed the escalation identity gate
// (contractBlocksShareIdentity, scoping constraint 4) as a WHOLE-STRING compare
// of normalizeReasonForFingerprint(reason). Its audit rejected that HIGH:
//
//	"contractBlocksShareIdentity compares whole summarize() strings, so a
//	 partially-repaired violation set (subset) reads as a different defect and
//	 suppresses [escalation]"
//	(.evolve/runs/cycle-1289/audit-fail-reason.json)
//
// The compared string is deliverable.summarize() — a "; "-joined rendering of
// EVERY violation on the block ("[code] message"). So a correction that closes
// one of two violations makes block 2 render a strict SUBSET of block 1, the two
// strings differ verbatim, and the escalation is suppressed at exactly the
// moment the CLI has most clearly proven it cannot repair the deliverable. The
// fix is to key identity on the violation-CODE SET (deliverable.Violation.Code,
// the stable primitive) rather than on rendered text: same defect ⇔ the two
// blocks' code sets intersect.
//
// IMPORT-CYCLE CONSTRAINT (cycle-644 reachability obligation, compiler-verified
// this cycle): internal/deliverable imports internal/core (reviewer.go:12,
// verifier.go:18) and core imports deliverable nowhere, so core.ReviewResult can
// NOT carry []deliverable.Violation — that shape is an import cycle and would
// make the criterion permanently unsatisfiable. The code set must reach core as
// plain data. These predicates pin BEHAVIOUR through the real RunCycle ladder
// and deliberately do NOT pin either implementation shape.
//
// PREDICATE STRATEGY. Every predicate executes the system under test through a
// `go test` subprocess on ONE named package narrowed by -run, and requires an
// explicit "--- PASS: <name>" per named test — exit 0 alone never satisfies a
// predicate (rename/skip gaming is caught). No source-grep predicate exists here
// (the cycle-85 degenerate-predicate ban): a "the file contains the word
// codeSet" assertion passes on a magic string regardless of the fix. The
// underlying unit tests drive Orchestrator.RunCycle → reviewAndGuard, the real
// production caller of contractBlocksShareIdentity, so a seam reachable only
// from a test cannot satisfy them.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 subset repair still escalates      → C1291_001  (POSITIVE — THE audit defect)
//	AC2 superset regression still escalates→ C1291_002  (POSITIVE)
//	AC3 disjoint sets do NOT escalate      → C1291_003  (NEGATIVE — anti-over-escalation)
//	AC4 reordered/reworded set escalates   → C1291_004  (EDGE — rendering instability)
//	AC5 code-less reasons still escalate   → C1291_005  (EDGE — fail-safe)
//	AC6 cycle-1289 identity gate preserved → C1291_006  (regression, 3 pre-existing tests)
//	AC7 escalation feature set intact      → C1291_007  (regression, full suite)
//	AC8 core + deliverable build and vet    → C1291_008
//	AC9 eval file passes quality-check      → C1291_009
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

// runGoTest runs the named tests of ONE package (verbose, fresh) and requires an
// explicit "--- PASS: <name>" for every wantPass. Deliberately -run-narrowed and
// single-package: a `./...` sweep or an unnarrowed ./internal/core run is the
// flaky-predicate shape that false-RED'd cycles 1173/1175/1178 under fleet load.
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

// AC1 — THE cycle-1289 audit defect. Block 1 reports {missing_section,
// missing_verdict}; the correction closes one, so block 2 reports the SUBSET
// {missing_verdict}. Same defect, partially repaired ⇒ the second consecutive
// block must still escalate off the failing CLI family.
func TestC1291_001_subset_repair_still_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_SubsetRepairStillEscalates$",
		[]string{"TestContractCorrection_SubsetRepairStillEscalates"})
}

// AC2 — the mirror direction: the correction left block 1's violation open AND
// added another, so block 2's code set is a SUPERSET. At least as strong an
// incapable-CLI signature; whole-string identity suppresses it identically.
func TestC1291_002_superset_regression_still_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_SupersetRegressionStillEscalates$",
		[]string{"TestContractCorrection_SupersetRegressionStillEscalates"})
}

// AC3 — NEGATIVE axis. Two multi-violation blocks sharing NO code are two honest
// defects (scoping constraint 4) and must NOT escalate. This is the predicate
// that fails an implementation which "fixes" AC1 by deleting the identity gate.
func TestC1291_003_disjoint_violation_sets_do_not_escalate(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_DisjointViolationSetsDoNotEscalate$",
		[]string{"TestContractCorrection_DisjointViolationSetsDoNotEscalate"})
}

// AC4 — EDGE axis on rendering instability. summarize() joins violations in
// slice order and quotes message text verbatim, so ONE defect set can render two
// ways. Code-set identity is invariant to order and wording; residual text-shaped
// comparison is not.
func TestC1291_004_reordered_violation_set_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_ReorderedViolationSetEscalates$",
		[]string{"TestContractCorrection_ReorderedViolationSetEscalates"})
}

// AC5 — EDGE fail-safe. Not every reason on this path is a summarize() rendering
// carrying "[code]" tokens. A code-set gate that reads two EMPTY sets as
// "different defect" would silently delete the escalation ladder for every
// non-summarize reason shape; two identical code-less reasons must still escalate.
func TestC1291_005_uncoded_reasons_fall_back_to_text_identity(t *testing.T) {
	runGoTest(t, corePkg, "^TestContractCorrection_UncodedReasonsFallBackToTextIdentity$",
		[]string{"TestContractCorrection_UncodedReasonsFallBackToTextIdentity"})
}

// AC6 — the cycle-1289 identity gate's own three axes must survive the rewrite:
// differing violations suppress, normalization-equal reasons escalate, and the
// HOT-BREAKER edge (no prior reason observed ⇒ escalate) stays open. That last
// one is why the rule is "prior reason known AND differing ⇒ suppress" rather
// than "equal ⇒ escalate", and a code-set rewrite can easily drop it.
func TestC1291_006_cycle1289_identity_axes_preserved(t *testing.T) {
	runGoTest(t, corePkg,
		"^TestContractCorrection_(DifferingBlockReasonsDoNotEscalate|NormalizedIdenticalReasonsEscalate|HotBreakerEscalatesOnFirstCorrection)$",
		[]string{
			"TestContractCorrection_DifferingBlockReasonsDoNotEscalate",
			"TestContractCorrection_NormalizedIdenticalReasonsEscalate",
			"TestContractCorrection_HotBreakerEscalatesOnFirstCorrection",
		})
}

// AC7 — the rest of the escalation feature (count gate, family test, allowed_clis
// guardrail, demotion WARN/ledger/autofile) must not regress while identity is
// rewritten. Narrowed to the two escalation test prefixes, not the whole package.
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

// AC8 — both packages on the fix's seam build and vet clean. Named explicitly
// rather than swept with ./... so a failure names the package that broke; this
// is also the compiler proof that no import cycle was introduced between core
// and deliverable while threading the code set (the cycle-644 shape).
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

// AC9 — the Step 6b permanent regression eval for this task classifies PASS and
// is not vacuous.
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
