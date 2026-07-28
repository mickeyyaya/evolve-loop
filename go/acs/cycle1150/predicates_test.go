//go:build acs

// Package cycle1150 materialises the cycle-1150 acceptance criteria for this
// lane's single triage-committed top_n task:
//
//	wire-docsfloor-verify-cli (M) — wire the ADR-0077 blocking-grade classifier
//	    `deliverable.VerifyBuildWithChangedPaths` (added cycle-1144, ZERO
//	    production callers) into `evolve phase verify build`, the self-check
//	    every phase prompt's Deliverable Contract tells the agent to run before
//	    declaring done. The changed-path set comes from a newly exported
//	    `core.ChangedWorktreePaths` — a projection of the existing unexported
//	    derivation the host-side docs-floor reviewer already uses, NOT a second
//	    implementation (ADR-0034 no-drift invariant).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1144
// precedent). Each predicate shells `go test -run` over the RED contract tests
// authored this cycle, every one of which drives the REAL CLI entry point
// (`runPhaseVerify`) over a REAL git worktree and asserts on exit codes and
// stderr, or calls the real `core.ChangedWorktreePaths`. None is a source-grep
// of production code (the cycle-85 degenerate-predicate ban); C1150_004 carries
// one supplementary source assertion, but its load-bearing half is the
// subprocess run.
//
// RED now: `evolve phase verify build` calls `deliverable.VerifyWithStage`
// (phase_verify.go:71), which never sees the diff — so an architecture-class
// build with no docs delta exits 0; and `core.ChangedWorktreePaths` does not
// exist, so internal/core's test package fails to compile.
package cycle1150

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
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. A compile failure in
// the target package (the RED signal before Builder implements) surfaces as a
// non-zero exit. code < 0 is a genuine launch failure (toolchain missing /
// killed by signal), not a test verdict, so it is a hard predicate error rather
// than a silent RED.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1150_001_ArchClassBuildWithoutDocsFailsTheSelfCheck — AC1, the primary
// rejection contract and the whole point of the task. Running `phase verify
// build` against a worktree whose diff touches a trust-kernel surface with no
// docs delta must exit 1 and name `missing_architecture_docs`, in both the
// human and the --json rendering. Today it exits 0.
func TestC1150_001_ArchClassBuildWithoutDocsFailsTheSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1|TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode")
	if !ok {
		t.Errorf("`evolve phase verify build` does not reject an undocumented architecture-class diff:\n%s", out)
	}
}

// TestC1150_002_DocsDeltaPassesTheSelfCheck — AC2, the anti-false-positive
// half. The SAME architecture-class diff must exit 0 once a docs/architecture/
// or runtime-reference.md delta rides along. A fake that always emits the
// violation (or that hardcodes the code string into phase_verify.go) dies here.
func TestC1150_002_DocsDeltaPassesTheSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg, "TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0")
	if !ok {
		t.Errorf("a documented architecture-class build must still pass the self-check:\n%s", out)
	}
}

// TestC1150_003_OrdinaryCyclesAreUnaffected — AC3, the NEGATIVE / edge axis and
// the regression guard for every non-architecture cycle: test-only and ordinary
// code diffs inside a worktree, no --worktree at all (missing artifact still
// fails naming its path, floor stays silent), and a non-build phase supplied
// the same architecture-class worktree.
func TestC1150_003_OrdinaryCyclesAreUnaffected(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0|TestPhaseVerify_NoWorktree_ByteIdentical|TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor")
	if !ok {
		t.Errorf("the wiring must be fail-open for non-architecture diffs, absent worktrees and non-build phases:\n%s", out)
	}
}

// TestC1150_004_ChangedPathsAreOneSourceWithAProjection — AC5, the
// no-duplication contract. The changed-path derivation must be the EXPORTED
// wrapper over core's existing logic, and the CLI must actually call the
// ADR-0077 seam function rather than growing its own git shell-out.
//
// Load-bearing assertion is the subprocess run (it CALLS core.ChangedWorktreePaths
// and asserts on returned values); the two source assertions are supplementary
// anti-gaming checks that the projection is the one being consumed — neither
// can pass on its own, C1150_001/002 still have to go green through the real
// classifier.
func TestC1150_004_ChangedPathsAreOneSourceWithAProjection(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestChangedWorktreePaths_ExportedForCLIConsumers|TestChangedWorktreePaths_EmptyWorktreeIsEmpty")
	if !ok {
		t.Errorf("core.ChangedWorktreePaths is not exported with the tracked+untracked semantics the CLI needs:\n%s", out)
	}

	verifySrc := filepath.Join(acsassert.RepoRoot(t), "go/internal/cli/phasecmd/phase_verify.go")
	src, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", verifySrc, err)
	}
	for _, needle := range []string{"VerifyBuildWithChangedPaths", "ChangedWorktreePaths"} {
		if !strings.Contains(string(src), needle) {
			t.Errorf("phase_verify.go must consume %s — a classifier nobody calls is the exact defect this cycle closes", needle)
		}
	}
}

// TestC1150_005_TouchedPackagesStayGreen — AC4, no-regression. Threading the
// changed-path set through the verify call site touches three packages; all of
// them must stay green, not just the new tests.
func TestC1150_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{
		phasecmdPkg,
		corePkg,
		"github.com/mickeyyaya/evolve-loop/go/internal/deliverable",
	} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		out := stdout + stderr
		if code < 0 {
			t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", pkg, code, err, out)
		}
		if code != 0 {
			t.Errorf("%s must stay green after the wiring:\n%s", pkg, out)
		}
	}
}
