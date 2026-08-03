//go:build acs

// Package cycle1253 materialises the cycle-1253 acceptance criteria for the one
// task this fleet lane committed: `tia-importer-closure` (triage top_n; the two
// sibling tasks `acssuite-scoped-regression-selection` and
// `tia-selection-wiring-proof` are DEFERRED and carry ZERO predicates here, per
// the R9.3 floor-binding rule).
//
// The task. changedpkgs derives changed-package selection FORWARD-ONLY: a
// changed file maps to the package it lives in, never to packages that import
// it. That is the cycle-1250 miss — a change confined to `internal/router` never
// selected `internal/routingtest`, which owns the keystone parity invariant, and
// main stayed red for 5 commits. Builder adds
// `changedpkgs.ImporterClosure(repoRoot string, pkgs []string) []string`: the
// sorted, deduped union of the input patterns and every module package that
// TRANSITIVELY imports one of them, best-effort (bad repoRoot / go-list failure
// → input unchanged, never a panic, never a lost entry).
//
// Predicate strategy — every predicate below EXECUTES the system under test via
// its unit suite in a single named package (`./internal/changedpkgs`), never a
// source-grep of production code (the cycle-85 degenerate-predicate ban) and
// never a `/...` test sweep (the flaky-predicate-shape ban):
//
//   - 001 is the crux: the cycle-1250 reproducer (router → routingtest) PLUS the
//     anti-no-op negative (a return-everything closure must fail).
//   - 002 pins transitivity (2-hop), the best-effort/no-panic contract, and the
//     sorted+deduped output shape.
//   - 003 is the apicover two-signal check scoped to this one package: the new
//     export must be NAMED and genuinely EXERCISED (non-zero coverage), not
//     merely present. `internal/changedpkgs` is enrolled in go/.apicover-enforce
//     (line 35), so an uncovered new export reddens the repo-wide gate.
//   - 004 is the blast-radius guard: the module still builds and the changed
//     package vets clean.
//
// Named unit tests are asserted by their `--- PASS:` line, so a renamed, skipped,
// or never-authored test cannot satisfy a predicate by a green package exit.
package cycle1253

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// changedpkgsPkg is the single named package under test. Full module path so
	// the predicate is independent of the acs package's cwd.
	changedpkgsPkg = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	// modulePkg is used for the build-only blast-radius check (never `go test`).
	modulePkg = "github.com/mickeyyaya/evolve-loop/go/..."
)

// runGoTest runs one named package's tests filtered to runExpr and asserts each
// wantPass test reported PASS. Asserting the individual `--- PASS:` lines (not
// just the package exit code) is what makes a renamed or skipped test a RED.
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
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s",
				name, stdout)
		}
	}
}

// -----------------------------------------------------------------------------
// AC1 + AC2 — the cycle-1250 reproducer, and the anti-no-op negative.
// -----------------------------------------------------------------------------

// TestC1253_001_ImporterClosureReproducer is the crux predicate. It executes the
// reproducer (a change in internal/router must select internal/routingtest,
// which imports it) together with its negative twin (internal/gitexec, a leaf
// that cannot transitively import router, must NOT be selected). The negative is
// what kills the degenerate "return every package" implementation that would
// otherwise satisfy the reproducer while making selection worthless.
func TestC1253_001_ImporterClosureReproducer(t *testing.T) {
	runGoTest(t, changedpkgsPkg,
		"^TestImporterClosure_(RouterRoutingtest|ExcludesNonImporters)$",
		[]string{
			"TestImporterClosure_RouterRoutingtest",
			"TestImporterClosure_ExcludesNonImporters",
		})
}

// -----------------------------------------------------------------------------
// AC3 + AC4 + AC5 — transitivity, best-effort contract, output shape.
// -----------------------------------------------------------------------------

// TestC1253_002_TransitiveBestEffortAndShape executes the remaining behavioural
// contract: the closure is transitive rather than one-hop (gitexec → changedpkgs
// → acssuite); every degenerate input (empty/nonexistent/non-repo root, nil and
// empty pkgs, a junk pattern) returns the input unchanged without panicking, so
// selection can never NARROW below the forward-only baseline; and the output is
// sorted + deduped with "./..." as a fixed point.
func TestC1253_002_TransitiveBestEffortAndShape(t *testing.T) {
	runGoTest(t, changedpkgsPkg,
		"^TestImporterClosure_(Transitive|BestEffortOnBadInput|SortedDedupedAndModuleRoot)$",
		[]string{
			"TestImporterClosure_Transitive",
			"TestImporterClosure_BestEffortOnBadInput",
			"TestImporterClosure_SortedDedupedAndModuleRoot",
		})
}

// -----------------------------------------------------------------------------
// AC6 — the new export is genuinely exercised (apicover two-signal, scoped).
// -----------------------------------------------------------------------------

// TestC1253_003_NewExportCovered asserts the new exported symbol is not just
// named but EXECUTED: it builds a coverage profile for the single changed
// package and requires ImporterClosure to report non-zero coverage.
// internal/changedpkgs is enrolled in go/.apicover-enforce, so a new export that
// no test drives is a repo-wide gate RED — this catches it inside the cycle
// instead of on main.
func TestC1253_003_NewExportCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-coverprofile="+profile, changedpkgsPkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -coverprofile %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			changedpkgsPkg, code, err, stdout, stderr)
	}

	funcs, ferr, fcode, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if fcode != 0 || err != nil {
		t.Fatalf("go tool cover -func exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			fcode, err, funcs, ferr)
	}

	var line string
	for _, l := range strings.Split(funcs, "\n") {
		if strings.Contains(l, "ImporterClosure") {
			line = strings.TrimSpace(l)
			break
		}
	}
	if line == "" {
		t.Fatalf("ImporterClosure absent from the coverage profile — the export does not exist or no test names it\n%s", funcs)
	}
	if strings.HasSuffix(line, "0.0%") {
		t.Errorf("ImporterClosure has 0.0%% coverage — named but never executed (apicover false-green): %q", line)
	}
}

// -----------------------------------------------------------------------------
// AC7 — blast radius: the module still builds, the changed package vets clean.
// -----------------------------------------------------------------------------

// TestC1253_004_ModuleBuildsAndVets pins that the new export broke no caller and
// introduced no vet-visible defect. Build-only over the module (never a `go test`
// sweep) plus `go vet` scoped to the one changed package.
func TestC1253_004_ModuleBuildsAndVets(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}

	stdout, stderr, code, err = acsassert.SubprocessOutput("go", "vet", changedpkgsPkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			changedpkgsPkg, code, err, stdout, stderr)
	}
}
