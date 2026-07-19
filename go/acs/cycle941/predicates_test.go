//go:build acs

// Package cycle941 materializes the cycle-941 acceptance criteria for the
// fleet-scoped todo merge-rung2-scoped-merge-review — the merge ladder's
// missing RUNG 2 (scoped merge review), between RUNG 0's trivial-rebase
// carry-forward and RUNG 3's full re-audit (knowledge-base/research/
// merge-concurrency-2026, MergeBERT lineage).
//
// Two committed tasks (triage top_n):
//   - merge-rung2-scoped-review-core: the pure decision core in
//     go/internal/core/mergerung2.go + the `Method` field on the ledger's
//     CompositionVerdictInput.
//   - merge-rung2-wire-ship-recovery: the re-entry (patch-id) invariant and
//     the orchestrator wiring observability.
//
// Every predicate executes the system under test via a `go test` subprocess
// requiring an explicit "--- PASS: <name>" for each named unit test (rename/
// skip gaming is caught — exit 0 alone never satisfies a predicate), or via a
// whole-module build/vet, or the SSOT eval quality checker. No source-grep
// predicates (cycle-85 rule). Adversarial axes are carried by the underlying
// unit tests: NEGATIVE (TestScopedReview_EmptyIntersectionNoDispatch — reviewer
// must NOT fire, armed to return entangled), EDGE (…MalformedDiffFailsClosed —
// corrupt hunk header fail-closed), SEMANTIC (only-intersecting, compatible-vs-
// entangled, patch-id re-entry, and method persistence are distinct behaviors).
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	A1 only-intersecting-hunks dispatched   → C941_001 (core, SeesOnlyIntersectingHunks)
//	A2 compatible composes / entangled esc. → C941_002 (core, CompatibleComposesEntangledEscalates)
//	A3 negative: empty intersection no-op   → C941_003 (core, EmptyIntersectionNoDispatch)
//	A4 edge: malformed diff fail-closed     → C941_004 (core, MalformedDiffFailsClosed)
//	A5 ledger Method field (scoped + default)→ C941_005 (ledger, Method{ScopedReview,DefaultsTrivialRebase})
//	B1 orchestrator wiring observability    → C941_006 (core, ScopedMergeReviewWired)
//	B2 MergeBERT patch-id re-entry invariant→ C941_007 (core, TestLLMResolution_ReentersRung0Verification)
//	A6/B-build whole module builds + vets   → C941_008 (go build ./...), C941_009 (go vet core+ledger)
//	Step 6b eval files pass quality-check    → C941_010 (both task slugs)
package cycle941

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	ledgerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	modulePkg = "github.com/mickeyyaya/evolve-loop/go/..."

	coreTaskSlug = "merge-rung2-scoped-review-core"
	wireTaskSlug = "merge-rung2-wire-ship-recovery"
)

// runGoTest runs the named tests of pkg (verbose, fresh) and requires an
// explicit "--- PASS: <name>" for every wantPass — exit 0 alone never
// satisfies a predicate (rename/skip gaming).
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

// A1: scoped review dispatches ONLY the intersecting hunks to the reviewer.
func TestC941_001_only_intersecting_hunks_dispatched(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_SeesOnlyIntersectingHunks$",
		[]string{"TestScopedReview_SeesOnlyIntersectingHunks"})
}

// A2: the reviewer's compatible/entangled disposition is carried through.
func TestC941_002_compatible_composes_entangled_escalates(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_CompatibleComposesEntangledEscalates$",
		[]string{"TestScopedReview_CompatibleComposesEntangledEscalates"})
}

// A3 (negative): an empty intersection does NOT invoke the reviewer and yields
// compatible — no wasted review, no false entanglement.
func TestC941_003_empty_intersection_no_dispatch(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_EmptyIntersectionNoDispatch$",
		[]string{"TestScopedReview_EmptyIntersectionNoDispatch"})
}

// A4 (edge): a malformed diff is fail-closed — error, reviewer not invoked,
// not reported compatible.
func TestC941_004_malformed_diff_fails_closed(t *testing.T) {
	runGoTest(t, corePkg, "^TestScopedReview_MalformedDiffFailsClosed$",
		[]string{"TestScopedReview_MalformedDiffFailsClosed"})
}

// A5: the ledger's CompositionVerdictInput.Method persists scoped-review and
// defaults blank → trivial-rebase (zero rung-0 regression).
func TestC941_005_ledger_method_field(t *testing.T) {
	runGoTest(t, ledgerPkg,
		"^TestWriteCompositionVerdict_Method(ScopedReview|DefaultsTrivialRebase)$",
		[]string{
			"TestWriteCompositionVerdict_MethodScopedReview",
			"TestWriteCompositionVerdict_MethodDefaultsTrivialRebase",
		})
}

// B1: WithScopedMergeReviewer + ScopedMergeReviewWired observability (nil
// default off = no regression).
func TestC941_006_orchestrator_scoped_review_wired(t *testing.T) {
	runGoTest(t, corePkg, "^TestOrchestrator_ScopedMergeReviewWired$",
		[]string{"TestOrchestrator_ScopedMergeReviewWired"})
}

// B2 (MergeBERT invariant): an LLM-assisted resolution re-enters RUNG 0 patch-id
// verification — matched by patch-id is trusted, a different resolution is
// rejected, a malformed resolution fails closed.
func TestC941_007_llm_resolution_reenters_rung0(t *testing.T) {
	runGoTest(t, corePkg, "^TestLLMResolution_ReentersRung0Verification$",
		[]string{"TestLLMResolution_ReentersRung0Verification"})
}

// A6/B-build: the whole module builds (no unused imports / dead code / broken
// callers from the new field + core API).
func TestC941_008_module_builds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}
}

// A6/B-vet: go vet clean on both touched packages.
func TestC941_009_vet_touched_packages_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", corePkg, ledgerPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			corePkg, ledgerPkg, code, err, stdout, stderr)
	}
}

// Step 6b: BOTH committed tasks' eval files exist and pass the SSOT quality
// checker with a NON-EMPTY command set (a missing/empty command block would
// PASS vacuously — the existence-check gaming the eval gate forbids).
func TestC941_010_eval_files_pass_quality_check(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, slug := range []string{coreTaskSlug, wireTaskSlug} {
		evalPath := filepath.Join(root, ".evolve", "evals", slug+".md")
		res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
		if err != nil {
			t.Fatalf("eval quality-check %s: %v", evalPath, err)
		}
		if res.Overall != evalqualitycheck.LevelPass {
			for _, c := range res.Commands {
				if c.Level != evalqualitycheck.LevelPass {
					t.Errorf("eval %s command %q classified level %d: %s", slug, c.Line, c.Level, c.Reason)
				}
			}
			t.Fatalf("eval %s overall level %d, want PASS(0)", slug, res.Overall)
		}
		if len(res.Commands) < 2 {
			t.Errorf("eval %s classified only %d command(s) — a vacuous eval is not a PASS", slug, len(res.Commands))
		}
	}
}
