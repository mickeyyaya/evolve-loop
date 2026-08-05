//go:build acs

// Package cycle1313 materializes the cycle-1313 acceptance criteria for this
// fleet lane's sole committed inbox item,
// triage-commit-time-protected-surface-admission (per R9.3 no predicates bind
// to any other lane's items).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy +
// Acceptance Criteria Summary):
//
//	AC1 Classify FAILs any triage artifact whose top_n card `files=` (both
//	    the brace-delimited and bare encodings actually observed in real
//	    triage-report.md output) intersects guards.IsProtectedSurface, citing
//	    the offending id and path in the diagnostic, and correctly
//	    disambiguates the offending card among several.
//	    → C1313_001 runs the three Reject* unit tests as a subprocess and
//	      requires each named "--- PASS:" marker (exit-0 alone could hide a
//	      renamed/skipped test).
//	AC2 A non-protected top_n card is unaffected (byte-identical PASS
//	    behavior preserved), including a card with no files= segment at all.
//	    → C1313_002 same subprocess pattern over the two regression/edge
//	      unit tests.
//	AC3 Existing console_routed_prompt_test.go / triage_test.go suites remain
//	    green (no behavior change to the prompt-composition route).
//	    → C1313_003 runs the full triage package suite (excluding the acs
//	      tag) and asserts a clean exit.
//	AC4 A permanent regression entry
//	    (.evolve/evals/triage-commit-time-protected-surface-admission.md)
//	    exists and passes the SSOT quality checker with a non-empty,
//	    real-command evidence set.
//	    → C1313_004 runs internal/evalqualitycheck — the exact code behind
//	      `evolve eval quality-check` — and requires Overall==PASS over ≥2
//	      commands, closing the vacuous-empty-eval hole.
//
// Adversarial axes: negative (Reject* tests assert FAIL on a protected
// path, both files= encodings), edge (no files= segment at all must not
// false-positive; multi-card batch must not misattribute the offender),
// semantic (PASS-unaffected vs FAIL-on-protected are distinct behaviors, not
// one behavior restated). No source-grep predicates (cycle-85 rule): every
// predicate here executes the system under test as a subprocess or runs the
// SSOT checker — none asserts on source-file text alone.
package cycle1313

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	evalSlug  = "triage-commit-time-protected-surface-admission"
	triagePkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

// rejectTestNames is the negative/semantic AC surface: Classify must FAIL a
// top_n card naming a protected path (both files= encodings) and must
// disambiguate the offending card among several. Each must report an
// explicit verbose PASS — count is asserted, never assumed from exit 0.
var rejectTestNames = []string{
	"TestTriageClassify_RejectsProtectedSurfaceTopNCard_BraceSyntax",
	"TestTriageClassify_RejectsProtectedSurfaceTopNCard_BareSyntax",
	"TestTriageClassify_RejectsAmongMultipleCards_NamesOffendingIdOnly",
}

// allowTestNames is the regression/edge AC surface: a non-protected card
// (and a card with no files= segment at all) must be unaffected.
var allowTestNames = []string{
	"TestTriageClassify_AllowsNonProtectedTopNCard",
	"TestTriageClassify_NoFilesSegmentIsUnaffected",
}

// AC1: the negative/semantic admission-check contract is green in THIS tree.
func TestC1313_001_reject_protected_surface_contract_green(t *testing.T) {
	pattern := "TestTriageClassify_(RejectsProtectedSurfaceTopNCard_BraceSyntax|RejectsProtectedSurfaceTopNCard_BareSyntax|RejectsAmongMultipleCards_NamesOffendingIdOnly)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, triagePkg, code, err, stdout, stderr)
	}
	for _, name := range rejectTestNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("admission-check test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

// AC2: the non-protected / no-files-segment cards remain unaffected.
func TestC1313_002_non_protected_cards_unaffected_contract_green(t *testing.T) {
	pattern := "TestTriageClassify_(AllowsNonProtectedTopNCard|NoFilesSegmentIsUnaffected)"
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", pattern, triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pattern, triagePkg, code, err, stdout, stderr)
	}
	for _, name := range allowTestNames {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("regression test %s did not report PASS (renamed, skipped, or not run)", name)
		}
	}
}

// AC3: the full triage package suite (prompt composition + classify) stays
// green — the new admission check must not regress route #1 (inbox-side
// PartitionConsole screen) or any pre-existing Classify behavior.
func TestC1313_003_triage_package_suite_green(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", triagePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			triagePkg, code, err, stdout, stderr)
	}
}

// AC4: the permanent eval entry passes the SSOT quality checker with a
// NON-EMPTY command set (an eval with no classifiable commands PASSes
// vacuously — that hole is closed here).
func TestC1313_004_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}
