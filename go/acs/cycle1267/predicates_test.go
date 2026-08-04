//go:build acs

// Package cycle1267 materialises the cycle-1267 acceptance criteria for the two
// tasks triage committed to this lane:
//
//   - scope-test-amplification-context            (inbox test-amplification-context-scope, w=0.89)
//   - verify-infra-teardown-predicate-consolidation (inbox infra-teardown-predicate-single-source, w=0.86)
//
// Scope note (read before judging these predicates). Both items arrived at this
// cycle PARTIALLY LANDED, carried in by the ADR-0076 continuation snapshot
// 71df9088. The TDD phase verified the live tree rather than trusting the scout
// report and pinned only what is genuinely open:
//
//	Task 1 — CoveringTests + its artifact + the phase-spec input + the persona
//	         instruction all exist and are green. Still MISSING: the inbox
//	         item's "+ direct reverse-import test packages" half (no
//	         reverse-dependency seam exists in the module at all — the cycle-1267
//	         fault-localization report cites changedpkgs.ImporterClosure, which
//	         does NOT exist), and the "no silent caps" half (the cap writes a
//	         note into the artifact but nothing reaches the operator's log).
//	Task 2 — verification-only by triage's own decision. The union-uniqueness
//	         proof already exists; the item's acceptance criterion "NO
//	         timeout-only or transient-only site was incorrectly widened" had no
//	         pin for the TIMEOUT-only half. 004/005 are that pin plus the
//	         existing uniqueness scan as a regression guard, and they are
//	         PRE-EXISTING GREEN by design: a behaviour-preserving task's
//	         deliverable is a durable proof, not a diff.
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - A source-grep predicate (FileContains over a .go file) is deliberately
//     avoided — it passes the moment the magic string appears, fix or no fix
//     (the cycle-85 degenerate-predicate ban).
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the two that name ./internal/core (a known 40s+ suite)
//     are narrowed with -run, which the rule explicitly permits. No wall-clock
//     bounds, no literal PIDs, no bare `git`, no un-reaped load generators.
package cycle1267

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	corePkg        = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed
// a `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate
// always exercises current source.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 means the subprocess never launched (toolchain/module resolution
		// failure) — a harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC1267_001_DirectImportersWidensCorpusToCoveringTestPackages — AC1-AC5 of
// scope-test-amplification-context, the half of the inbox item's how_to_apply
// that never landed: "their *_test.go files + DIRECT REVERSE-IMPORT test
// packages". Exercises the real deriver over real on-disk module trees: a
// package that imports the changed package only from its *_test.go is a
// covering test and must be widened in; transitive importers, unrelated
// packages, a same-suffix package from a different module, and the input
// packages themselves must stay out; the output is sorted, deduped and
// deterministic; and every unusable input fails open to nil so the corpus
// degrades to today's changed-packages-only set instead of blocking the phase.
func TestC1267_001_DirectImportersWidensCorpusToCoveringTestPackages(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestDirectImporters_WidensToReverseImportersIncludingTestOnly",
		"TestDirectImporters_AcceptsBothPatternForms",
		"TestDirectImporters_DeterministicSortedAndDeduped",
		"TestDirectImporters_FailsOpenOnUnusableInput",
		"TestDirectImporters_NoImportersIsNotAnError",
	)
}

// TestC1267_002_DirectImportersReachableFromProduction — AC6, the WIRING proof.
// The widening must be called from a real non-test file in the go/ module
// (resolved from the parsed import graph, not a grep). A seam whose only caller
// is a test injects nothing into test-amplification's context and saves zero
// tokens — the exact dead-code shape the cycle-1255 CoveringTests contract had
// to pin for the same reason.
func TestC1267_002_DirectImportersReachableFromProduction(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestDirectImporters_ReachableFromProduction",
	)
}

// TestC1267_003_CoveringTestsCapIsLoudNotSilent — AC7-AC8, the "no silent caps"
// half of the item. The renderer must REPORT the number of dropped paths (one
// source of truth for both the in-artifact note and the operator warning), and
// the real write seam — driven over a real git worktree whose corpus overflows
// the cap — must emit that count to stderr, while staying silent when nothing
// was dropped. Without this an operator cannot tell a corpus trimmed to 60%
// from one injected whole, which makes the item's own before/after token
// measurement uninterpretable.
func TestC1267_003_CoveringTestsCapIsLoudNotSilent(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
	)
}

// TestC1267_004_TimeoutOnlySitesNotWidenedToUnion — AC9-AC10 of
// verify-infra-teardown-predicate-consolidation: the acceptance criterion "NO
// site that is timeout-only or transient-only was incorrectly widened to the
// union predicate", which the item calls its whole risk and which had no pin
// for the TIMEOUT-only half. Behavioural for the observable site
// (writePhaseFailureDiag must map ErrArtifactTimeout to exit 81 and must NOT
// relabel a transient bridge failure as 81) and structural for the site
// reachable only through a fully wired orchestrator.
func TestC1267_004_TimeoutOnlySitesNotWidenedToUnion(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestWritePhaseFailureDiag_TimeoutOnlyNotWidened",
		"TestTimeoutOnlySites_NotWidenedToUnion",
	)
}

// TestC1267_005_InfraTeardownUnionStillSpelledExactlyOnce — AC11, the durable
// single-source proof this task's other half is about. Re-runs the existing
// uniqueness scan so the consolidation cannot silently regress while Task 1
// edits the same package: exactly one function in internal/core may spell
// (timeout OR transient), and it must be IsInfraTeardownError.
func TestC1267_005_InfraTeardownUnionStillSpelledExactlyOnce(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestInfraTeardownUnion_SpelledExactlyOnce",
		"TestIsInfraTeardownError_UnionSemantics",
		"TestIsTransientBridgeError_StaysTransientOnly",
	)
}

// TestC1267_006_CoveringTestsContractNotRegressedByWidening — the anti-regression
// AC. The widening is ADDITIVE: CoveringTests keeps its cycle-1255 contract
// (changed packages → their own _test.go files, both pattern forms, deduped,
// fail-open on the module-wide sweep). "Fixing" the missing reverse-import half
// by making CoveringTests itself walk the whole module — the blind-widen shape
// that would re-inflate the very context this task shrinks — turns this RED.
func TestC1267_006_CoveringTestsContractNotRegressedByWidening(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_DedupesAcrossOverlappingPatterns",
		"TestCoveringTests_AcceptsNonRecursivePatternForm",
		"TestCoveringTests_FailsOpenOnUnusableInput",
		"TestCoveringTests_ReachableFromProduction",
	)
}
