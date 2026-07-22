//go:build acs

// Package cycle1067 materialises the cycle-1067 acceptance criteria for this
// lane's single fleet-scoped task:
//
//   - ship-stage-explicit-paths → shipDirect (gitops.go:228) and
//     shipFromWorktree (gitops.go:374) must stop staging with `git add -A` for
//     the non-release classes and stage an explicit `git add -- <paths>`
//     instead, sourced from the DECLARED manifest (build-report.md +
//     test-report.md, the set declaredManifest already computes for the
//     manifest gate), with a porcelain-changed-set fallback when no manifest is
//     readable. The release class already proves the pattern (stageReleaseSet,
//     gitops.go:707).
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban). The staging call sites are unexported, so
// each predicate shells the ship package's OWN behavioural tests, which drive
// shipDirect / shipFromWorktree through their production code paths:
//
//   - 001 asserts the declared-manifest staging effect (git args captured from a
//     real shipDirect run: explicit pathspec, undeclared stray excluded).
//   - 002 asserts the manifest-empty FALLBACK — the H2 blast-radius risk: an
//     empty manifest must fall back to the porcelain changed set, never to
//     `add -A` and never to a silent skip (which would ship nothing while
//     reporting success).
//   - 003 is the negative/anti-no-op axis: NO non-release class may emit
//     `git add -A`, and the stale discriminator test name asserting the old
//     behaviour must no longer exist.
//   - 004 is the real-git effect check (integration tag): the ship commit
//     produced against a genuine repository contains the declared path and NOT
//     the undeclared untracked stray `add -A` would sweep in.
//   - 005 is the no-regression floor: the whole ship package, N/N PASS.
//
// No predicate asserts on production source text.
package cycle1067

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// shipPkg is the package under test.
const shipPkg = "./internal/phases/ship/"

// goDir is the worktree's Go module root — the -C target, so the shelled lanes
// compile the CYCLE's tree, not main's stale copy.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// runGoTest runs one named test in the worktree module and requires a real PASS
// line (a filtered-away or renamed test exits 0 with no PASS — that is a FAIL
// here, not a silent green). extraArgs carries build tags where needed.
func runGoTest(t *testing.T, name string, extraArgs ...string) {
	t.Helper()
	args := append([]string{"test", "-C", goDir(t), "-count=1", "-v"}, extraArgs...)
	args = append(args, "-run", "^"+name+"$", shipPkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", shipPkg, name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s (renamed, skipped, or never ran?)\n%s", name, out)
	}
}

// TestC1067_001_CycleShipStagesDeclaredPathsNotAddAll — AC1 + the declared-path
// sourcing: a real shipDirect run under ClassCycle must emit an explicit
// `git add -- <declared paths>` (never `add -A`) and must leave an undeclared
// sibling-lane stray out of the pathspec.
func TestC1067_001_CycleShipStagesDeclaredPathsNotAddAll(t *testing.T) {
	runGoTest(t, "TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll")
}

// TestC1067_002_EmptyManifestFallsBackToChangedSetNeverSkips — AC2 (H2): with
// no readable phase reports, and with no WorkspacePath at all, staging falls
// back to the porcelain changed set. A fix that silently skips staging (empty
// pathspec → false clean exit / empty ship) fails here.
func TestC1067_002_EmptyManifestFallsBackToChangedSetNeverSkips(t *testing.T) {
	runGoTest(t, "TestShipDirect_ManualClass_EmptyManifestFallsBackToChangedSet")
	runGoTest(t, "TestShipDirect_NoWorkspacePath_StillStagesExplicitly")
}

// TestC1067_003_NoNonReleaseClassEmitsAddAll — AC1 negative axis + AC3: every
// non-release class (cycle, manual, trivial) must be free of `git add -A`, and
// the stale discriminator TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll
// — which asserted the behaviour being removed — must no longer exist, while
// its still-valid churn-discard half survives under the new name.
func TestC1067_003_NoNonReleaseClassEmitsAddAll(t *testing.T) {
	runGoTest(t, "TestShipDirect_NonReleaseClasses_NeverAddAll")
	runGoTest(t, "TestShipDirect_CycleClass_KeepsChurnDiscard")

	// The stale name must be gone: `go test -run` on a nonexistent test exits 0
	// with no RUN line, so absence of the RUN line is the assertion.
	stale := "TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll"
	stdout, stderr, _, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", "^"+stale+"$", shipPkg)
	if err != nil {
		t.Fatalf("go test failed to launch: %v\n%s", err, stdout+stderr)
	}
	if strings.Contains(stdout+stderr, "=== RUN   "+stale) {
		t.Errorf("stale test %s still exists — it asserts `git add -A` staging that this cycle removed", stale)
	}
}

// TestC1067_004_WorktreeShipCommitExcludesUndeclaredStray — AC1 for the
// worktree path, verified as an OBSERVABLE EFFECT against a genuine repository
// (integration tag): the ship commit contains the declared path and not the
// undeclared untracked stray that `git add -A` sweeps in (cycle-645 leak).
func TestC1067_004_WorktreeShipCommitExcludesUndeclaredStray(t *testing.T) {
	runGoTest(t, "TestShipFromWorktree_StagesDeclaredPathsOnly_ExcludesUndeclaredStray",
		"-tags", "integration")
}

// TestC1067_005_ShipPackageSuiteGreen — AC5 no-regression floor: the whole ship
// package passes. The staging change sits in the commit-integrity critical
// section; a green crux with a reddened sibling test is not a ship.
func TestC1067_005_ShipPackageSuiteGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", shipPkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch: %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("ship package suite is RED (exit %d)\n%s", code, out)
	}
}
