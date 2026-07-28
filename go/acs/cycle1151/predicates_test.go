//go:build acs

// Package cycle1151 materialises the cycle-1151 acceptance criteria for this
// lane's two triage-committed top_n tasks:
//
//	wire-docsfloor-changed-paths-verify (S) — route the `build` phase's
//	    self-check through `deliverable.VerifyBuildWithChangedPathsStage` so the
//	    ADR-0077 blocking-grade classifier finally sees a real diff, fail-open
//	    everywhere else.
//	reconcile-adr0077-labeler-drift (S) — ADR-0077's "Shape" / boundary-2 prose
//	    still names `docsfloor.LabelArchitecture` as the label source while
//	    production (`core.docsFloorWarn`) calls `docsfloor.IsArchitectureClass`,
//	    and `LabelArchitecture` has zero production callers.
//
// Predicate strategy, per task:
//
//   - Task 1 is behavioural-via-subprocess (the cycle-549…1150 precedent): each
//     predicate shells `go test -run` over contract tests that drive the REAL
//     CLI entry point (`runPhaseVerify`) against a REAL git worktree and assert
//     on exit codes / JSON violation codes. No source-grep of production code
//     carries any load (the cycle-85 degenerate-predicate ban).
//   - Task 2's deliverable IS a document, so its predicates assert on that real
//     emitted artifact — but never against a hardcoded magic string. The
//     expected symbol is COMPUTED from production source (which call
//     `docsFloorWarn` actually makes) and the "is it unused" claim is COMPUTED
//     by walking the Go tree for callers. If production later rewires to
//     `LabelArchitecture`, these predicates flip to demanding the opposite text
//     — they detect drift, they do not pin a phrase.
//
// RED expectation at authoring time: Task 2's predicates (C1151_004, C1151_005)
// fail — the ADR still documents `LabelArchitecture` as the labeler and records
// nothing about its non-production status. Task 1's predicates (C1151_001…003)
// are PRE-EXISTING GREEN: the cycle-1150 continuation salvage (commit 0a328df1,
// carried into this lane's worktree base) already landed the wiring and its
// contract tests. They are retained as regression binding, and reported as
// pre-existing GREEN in test-report.md rather than claimed as this cycle's RED.
package cycle1151

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasecmdPkg = "github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	deliverPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

	adrRelPath      = "docs/architecture/adr/0077-docs-floor-for-architecture-changes.md"
	reviewerRelPath = "go/internal/core/build_floor_reviewer.go"

	strictLabeler = "IsArchitectureClass"
	broadLabeler  = "LabelArchitecture"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package surfaces as a non-zero exit (a legitimate RED). code < 0 is
// a genuine launch failure (toolchain missing / killed by signal), not a test
// verdict, so it is a hard predicate error rather than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// --- Task 1: wire-docsfloor-changed-paths-verify -----------------------------

// C1151_001 — an architecture-class build diff carrying no docs delta makes the
// live `evolve phase verify build` self-check FAIL with the
// `missing_architecture_docs` violation code. This is the whole point of the
// wiring: before it, the CLI called plain Verify and exited 0 on such a diff.
func TestC1151_001_ArchClassBuildWithoutDocsFailsSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1|TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode")
	if !ok {
		t.Errorf("architecture-class build diff with no docs delta does not fail `phase verify build`:\n%s", out)
	}
}

// C1151_002 — NEGATIVE / fail-open axis. The three shapes that must stay
// byte-identical to pre-wiring behaviour: an architecture-class diff that DOES
// carry docs, a non-architecture diff, an invocation with no --worktree, and a
// non-build phase. A wiring that fails these is over-reaching, which ADR-0077's
// "WARN, never REJECT" boundary forbids.
func TestC1151_002_FailOpenShapesAreUnchanged(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0|TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0|TestPhaseVerify_NoWorktree_ByteIdentical|TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor")
	if !ok {
		t.Errorf("fail-open behaviour regressed — the docs floor is taxing diffs it must ignore:\n%s", out)
	}
}

// C1151_003 — the seam the CLI now depends on holds its own contract: the
// changed-path derivation is exported from internal/core (one source, not a
// second implementation — the ADR-0034 no-drift invariant) and the deliverable
// helper threads the caller's resolver + PhaseIO stage rather than silently
// dropping to built-ins.
func TestC1151_003_SeamContractsHold(t *testing.T) {
	if ok, out := runGoTest(t, corePkg,
		"TestChangedWorktreePaths_ExportedForCLIConsumers|TestChangedWorktreePaths_EmptyWorktreeIsEmpty"); !ok {
		t.Errorf("core.ChangedWorktreePaths contract broken:\n%s", out)
	}
	if ok, out := runGoTest(t, deliverPkg,
		"TestVerifyBuildWithChangedPathsStage_AppliesFloorAndPreservesStage"); !ok {
		t.Errorf("VerifyBuildWithChangedPathsStage drops the caller's resolver/stage:\n%s", out)
	}
}

// --- Task 2: reconcile-adr0077-labeler-drift ---------------------------------

// productionLabeler reports which docsfloor labeler the live WARN call site
// actually invokes. COMPUTED from source, so these predicates track the code
// rather than pinning a phrase: rewire docsFloorWarn and the expectation moves
// with it.
func productionLabeler(t *testing.T) string {
	t.Helper()
	reviewer := filepath.Join(acsassert.RepoRoot(t), reviewerRelPath)
	strictN, err := acsassert.CountInGoFunc(reviewer, "docsFloorWarn", "docsfloor."+strictLabeler+"(")
	if err != nil {
		t.Fatalf("cannot inspect docsFloorWarn in %s: %v", reviewer, err)
	}
	broadN, err := acsassert.CountInGoFunc(reviewer, "docsFloorWarn", "docsfloor."+broadLabeler+"(")
	if err != nil {
		t.Fatalf("cannot inspect docsFloorWarn in %s: %v", reviewer, err)
	}
	switch {
	case strictN > 0 && broadN == 0:
		return strictLabeler
	case broadN > 0 && strictN == 0:
		return broadLabeler
	default:
		t.Fatalf("ambiguous production labeler in docsFloorWarn: %s=%d %s=%d", strictLabeler, strictN, broadLabeler, broadN)
		return ""
	}
}

// adrText reads the ADR under test.
func adrText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(acsassert.RepoRoot(t), adrRelPath))
	if err != nil {
		t.Fatalf("cannot read %s: %v", adrRelPath, err)
	}
	return string(b)
}

// section returns the ADR slice between two markers, or "" when absent.
func section(body, start, end string) string {
	i := strings.Index(body, start)
	if i < 0 {
		return ""
	}
	rest := body[i:]
	if j := strings.Index(rest[len(start):], end); j >= 0 {
		return rest[:len(start)+j]
	}
	return rest
}

// C1151_004 — the ADR's normative description of the label source must name the
// symbol production actually calls. Both places the drift lives are checked:
// boundary 2 ("The label is derived from the diff") and the "### Shape" API
// listing. Expectation is derived, not hardcoded.
func TestC1151_004_ADRNamesTheProductionLabeler(t *testing.T) {
	want := productionLabeler(t)
	body := adrText(t)

	boundary := section(body, "2. **The label is derived from the diff", "3. **SKIP is not PASS")
	if boundary == "" {
		t.Fatalf("ADR-0077 boundary-2 bullet not found — has the ADR been restructured?")
	}
	if !strings.Contains(boundary, want) {
		t.Errorf("ADR-0077 boundary 2 does not name the production labeler %q:\n%s", want, boundary)
	}

	shape := section(body, "### Shape", "Decision order:")
	if shape == "" {
		t.Fatalf("ADR-0077 \"### Shape\" section not found")
	}
	if !strings.Contains(shape, want) {
		t.Errorf("ADR-0077 \"### Shape\" API listing omits the production labeler %q:\n%s", want, shape)
	}
}

// countProductionCallers walks go/ and counts call sites of docsfloor.<sym>(
// outside _test.go files and outside the defining package itself. Real grep of
// the real tree — the "is it unused" claim is measured, never assumed.
func countProductionCallers(t *testing.T, sym string) int {
	t.Helper()
	root := filepath.Join(acsassert.RepoRoot(t), "go")
	needle := "docsfloor." + sym + "("
	n := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		n += strings.Count(string(b), needle)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return n
}

// noteNearMention reports whether any note appears within window bytes of any
// occurrence of mention. All inputs are expected lowercased by the caller.
func noteNearMention(body, mention string, notes []string, window int) bool {
	for off := 0; ; {
		i := strings.Index(body[off:], mention)
		if i < 0 {
			return false
		}
		i += off
		lo, hi := i-window, i+len(mention)+window
		if lo < 0 {
			lo = 0
		}
		if hi > len(body) {
			hi = len(body)
		}
		for _, n := range notes {
			if strings.Contains(body[lo:hi], n) {
				return true
			}
		}
		off = i + len(mention)
	}
}

// C1151_005 — NEGATIVE / drift axis. Given the measured fact that the broad
// labeler has zero production callers, the ADR must SAY so wherever it mentions
// it; and its boundary-2 bullet may not present the broad labeler as the live
// label source without also naming the strict one. If a future cycle wires
// LabelArchitecture into production, the caller count flips and this predicate
// correctly stops demanding the note — it asserts agreement between doc and
// tree, not a fixed sentence.
func TestC1151_005_ADRRecordsUnusedLabelerStatus(t *testing.T) {
	body := adrText(t)
	callers := countProductionCallers(t, broadLabeler)
	if callers > 0 {
		t.Skipf("%s now has %d production caller(s) — no non-production note is owed", broadLabeler, callers)
	}
	if !strings.Contains(body, broadLabeler) {
		return // scrubbed entirely — nothing left to mislabel
	}

	notes := []string{
		"no production caller",
		"zero production caller",
		"not called in production",
		"unused in production",
		"no live caller",
	}
	// Proximity-scoped: the note must sit NEXT TO a LabelArchitecture mention.
	// A bare document-wide substring search passes on the cycle-1150 addendum's
	// "shipped inert … zero production callers" sentence, which is about
	// VerifyBuildWithChangedPaths — a different symbol entirely.
	if !noteNearMention(strings.ToLower(body), strings.ToLower(broadLabeler), notes, 400) {
		t.Errorf("ADR-0077 still documents %s but records nothing about its measured %d production callers "+
			"within 400 chars of any mention; expected one of %v", broadLabeler, callers, notes)
	}

	boundary := section(body, "2. **The label is derived from the diff", "3. **SKIP is not PASS")
	if strings.Contains(boundary, broadLabeler) && !strings.Contains(boundary, strictLabeler) {
		t.Errorf("ADR-0077 boundary 2 presents %s as the live labeler without naming %s (the symbol production calls):\n%s",
			broadLabeler, strictLabeler, boundary)
	}
}
