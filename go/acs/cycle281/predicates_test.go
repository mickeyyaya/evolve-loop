//go:build acs

// Package cycle281 materializes the cycle-281 acceptance criteria for the three
// committed top_n tasks (scout-report.md — coverage + adversarial-testing
// campaign):
//
//	T1  fix-inserted-phase-worktree-dispatch — the cycle-280 P0: advisor-INSERTED
//	    write-capable phases dispatched with Worktree="" (mint default
//	    writes_source:false) → tree-diff guard cycle-fatal + abort-cleanup deletes
//	    the uncommitted worktree. Fix: a minted phase defaults to write-capable
//	    (inherits the worktree); an explicit writes_source:false stays read-only;
//	    an abnormal mid-cycle abort PRESERVES the worktree.
//	T2  adversarial-fault-injection-suite    — a real fault-injection suite (not the
//	    cycle-280 schema-only skeleton) driving fakeTmux scripted panes across the
//	    six fault types (stall / crash / update-menu / weak-busy / empty-pane /
//	    malformed) × the three driver families (claude / codex / agy).
//	T3  coverage-push-core-and-lower-packages — lift internal/core and
//	    internal/routingtest to >= 90% with intent-probing tests.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test as a real subprocess — `go test -v` over the core /
// bridge packages and `go tool cover` totals — and assert on the real
// `--- PASS: <name>` lines, sub-case counts, and coverage numbers the builder's
// tests produce. A magic string in a .go file can neither produce a named PASS
// line nor move a coverage number, so none of these is gameable by source editing
// alone (the established cycle-274/276 pattern).
//
// Convention: the TDD-engineer authors T1's three failing tests
// (orchestrator_inserted_worktree_test.go); the BUILDER authors T2's fault suite
// (adversarial_faults_test.go + its TestAdversarialFaultMatrix_* guards) and T3's
// coverage tests. These ACS predicates GATE on those tests running + passing with
// the required adversarial diversity. RED at baseline: T1 has two failing tests,
// T2's suite does not exist, and the coverage numbers sit below the floors.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.a TestInsertedPhaseWritableInheritsWorktree PASS  → C281_001
//	T1.b TestAbortCleanupPreservesWorktreeDiff PASS       → C281_001
//	T1.c TestInsertedReadOnlyPhaseDoesNotGetWorktree PASS → C281_001 (discriminator)
//	T2.a adversarial suite >= 18 passing cases            → C281_010
//	T2.b 6 fault types present as passing cases           → C281_011
//	T2.c 3 driver families present as passing cases       → C281_012
//	T2.d TestAdversarialFaultMatrix_* guards PASS         → C281_013
//	T3.a internal/core coverage >= 90%                    → C281_020
//	T3.b internal/routingtest coverage >= 90%             → C281_021
package cycle281

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// --- shared one-shot subprocess runners (one `go test` per scope, reused) ---

var (
	coreOnce sync.Once
	coreOut  string
	advOnce  sync.Once
	advOut   string
)

// runCoreWorktree runs ONLY the three T1 worktree-dispatch tests, verbose, ONCE
// per predicate process. Scoped via -run so an unrelated core regression cannot
// false-RED this gate.
func runCoreWorktree(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	coreOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestInsertedPhaseWritableInheritsWorktree|TestAbortCleanupPreservesWorktreeDiff|TestInsertedReadOnlyPhaseDoesNotGetWorktree",
			"./internal/core/")
		coreOut = stdout + "\n" + stderr
	})
	return coreOut
}

// runAdversarial runs the bridge package's adversarial fault suite ONCE. The
// builder's adversarial_faults_test.go (TestAdversarialFault* + the
// TestAdversarialFaultMatrix_* guards) lands here.
func runAdversarial(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	advOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "-run", "TestAdversarial", "./internal/bridge/")
		advOut = stdout + "\n" + stderr
	})
	return advOut
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// passNames returns every passing test path (top-level and sub) in `out`.
func passNames(out string) []string {
	var names []string
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

// topLevelPassed reports whether a `--- PASS: <name>` line names exactly `name`.
func topLevelPassed(out, name string) bool {
	for _, n := range passNames(out) {
		if n == name {
			return true
		}
	}
	return false
}

// faultCases returns the distinct passing test CASES under any TestAdversarialFault
// parent. A "case" is a sub-test (`Parent/sub`) OR a leaf top-level test with no
// sub-tests — so the count is identical whether the builder writes one
// table-driven test with N rows or N separate functions.
func faultCases(out string) []string {
	names := passNames(out)
	parents := map[string]bool{}
	for _, n := range names {
		if i := strings.Index(n, "/"); i >= 0 {
			parents[n[:i]] = true
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if !strings.HasPrefix(n, "TestAdversarialFault") {
			continue
		}
		if strings.Contains(n, "/") { // a sub-case
			seen[n] = true
			continue
		}
		if !parents[n] { // a leaf top-level test (no sub-rows)
			seen[n] = true
		}
	}
	out2 := make([]string, 0, len(seen))
	for n := range seen {
		out2 = append(out2, n)
	}
	return out2
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// coverageTotal runs the package suite with -coverprofile and returns the
// `total:` statement-coverage percentage `go tool cover -func` reports. The
// number is produced by REALLY running the package tests, so it can only move
// once the builder's new tests exercise real code — un-gameable by source edits.
func coverageTotal(t *testing.T, pkg string) (float64, string) {
	t.Helper()
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1", "-coverprofile="+prof, pkg)
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	for _, ln := range strings.Split(funcOut, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(ln), "total:") {
			continue
		}
		fields := strings.Fields(ln)
		pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
		if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
			return pct, ""
		}
	}
	return -1, "test stderr:\n" + tail(tErr, 20) + "\ncover stderr:\n" + tail(cErr, 20)
}

// ===================== T1 — inserted-phase worktree dispatch ==================

// --- C281_001 (T1.a/T1.b/T1.c): the inserted-phase worktree contract holds ---
//
// Behavioral: drives the real mint→register→runsInWorktree seam and the real
// RunCycle abort path via the three TDD-authored tests. Requiring all three named
// top-level PASS lines (and no FAIL) proves (a) a write-capable mint inherits the
// worktree, (b) an abnormal abort preserves the worktree, and (c) an explicit
// read-only mint still gets none — the discriminator that keeps the fix from
// blanket-granting worktrees. RED baseline: two of the three FAIL.
func TestC281_001_InsertedPhaseWorktreeContract(t *testing.T) {
	out := runCoreWorktree(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED: a worktree-dispatch test FAILs:\n%s", tail(out, 40))
	}
	for _, name := range []string{
		"TestInsertedPhaseWritableInheritsWorktree",
		"TestAbortCleanupPreservesWorktreeDiff",
		"TestInsertedReadOnlyPhaseDoesNotGetWorktree",
	} {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: %s did not PASS — the cycle-280 inserted-phase worktree contract is not yet satisfied", name)
		}
	}
}

// ===================== T2 — adversarial fault-injection suite =================

// --- C281_010 (T2.a): the suite drives >= 18 distinct fault cases ---
//
// The scout verifiableBy: `go test -run TestAdversarial` reports >= 18 sub-tests
// PASS. Counting DISTINCT passing cases (not RUN lines) proves they pass, not just
// dispatch. RED: no TestAdversarialFault* tests exist → 0 cases.
func TestC281_010_AdversarialSuiteHasMinimumCases(t *testing.T) {
	out := runAdversarial(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: adversarial suite has a FAIL line:\n%s", tail(out, 40))
	}
	cases := faultCases(out)
	if len(cases) < 18 {
		t.Errorf("RED: TestAdversarialFault* has %d passing case(s), want >= 18 "+
			"(6 fault types × >= 3 driver families — scout T2 verifiableBy)", len(cases))
	}
}

// --- C281_011 (T2.b): all six fault types are present as passing cases ---
//
// The strongest anti-no-op signal: a suite that only crashes-and-stalls does not
// exercise the update-menu/weak-busy/empty-pane/malformed branches the goal
// requires. Each fault type must appear (case-insensitively, hyphen/underscore
// agnostic) in a passing case name. RED: none present.
func TestC281_011_AdversarialSuiteCoversAllFaultTypes(t *testing.T) {
	out := runAdversarial(t)
	norm := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(s), "-", ""), "_", "")
	}
	cases := faultCases(out)
	// Each entry is the set of accepted spellings for one fault type.
	faultTypes := map[string][]string{
		"stall":       {"stall"},
		"crash":       {"crash"},
		"update-menu": {"updatemenu", "updatenag"},
		"weak-busy":   {"weakbusy"},
		"empty-pane":  {"emptypane"},
		"malformed":   {"malformed"},
	}
	for label, spellings := range faultTypes {
		found := false
		for _, c := range cases {
			nc := norm(c)
			for _, sp := range spellings {
				if strings.Contains(nc, sp) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("RED: no passing adversarial case covers fault type %q (accepted: %v) — fault dimension uncovered", label, spellings)
		}
	}
}

// --- C281_012 (T2.c): all three driver families are present as passing cases --
//
// The goal requires fault coverage across claude / codex / agy. Each family must
// appear in a passing case name. RED: none present.
func TestC281_012_AdversarialSuiteCoversAllDriverFamilies(t *testing.T) {
	out := runAdversarial(t)
	cases := faultCases(out)
	for _, fam := range []string{"claude", "codex", "agy"} {
		found := false
		for _, c := range cases {
			if strings.Contains(strings.ToLower(c), fam) {
				found = true
			}
		}
		if !found {
			t.Errorf("RED: no passing adversarial case covers driver family %q — family dimension uncovered", fam)
		}
	}
}

// --- C281_013 (T2.d): the matrix-invariant guards pass ---
//
// The builder's own coverage invariants (TestAdversarialFaultMatrix_Required*)
// assert family + fault-type completeness from inside the suite. Gating on them
// passing makes the suite self-policing against future fault-type/family drops.
// RED: the guards do not exist yet.
func TestC281_013_AdversarialMatrixGuardsPass(t *testing.T) {
	out := runAdversarial(t)
	for _, name := range []string{
		"TestAdversarialFaultMatrix_RequiredFamiliesCovered",
		"TestAdversarialFaultMatrix_RequiredFaultTypesPresent",
	} {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: matrix-invariant guard %s did not PASS — the suite is not self-policing for completeness", name)
		}
	}
}

// ===================== T3 — coverage floors ==================================

// --- C281_020 (T3.a): internal/core reaches the >= 90% floor ---
//
// Objective/un-gameable: the number comes from really running the core suite with
// -coverprofile. RED baseline (scout-281): internal/core = 86.2%.
func TestC281_020_CoreCoverageFloor(t *testing.T) {
	pct, diag := coverageTotal(t, "./internal/core/")
	if pct < 0 {
		t.Fatalf("RED: no `total:` row from `go tool cover -func` for internal/core — profile not produced.\n%s", diag)
	}
	if pct < 90.0 {
		t.Errorf("RED: internal/core coverage = %.1f%%, want >= 90.0%% (baseline 86.2%%; failure_advisor 0%% + correction_ladder 15-70%% must be probed)", pct)
	}
}

// --- C281_021 (T3.b): internal/routingtest reaches the >= 90% floor ---
//
// RED baseline (scout-281): internal/routingtest = 80.7% (several 0% funcs in
// engine/bricks/agent).
func TestC281_021_RoutingtestCoverageFloor(t *testing.T) {
	pct, diag := coverageTotal(t, "./internal/routingtest/")
	if pct < 0 {
		t.Fatalf("RED: no `total:` row from `go tool cover -func` for internal/routingtest — profile not produced.\n%s", diag)
	}
	if pct < 90.0 {
		t.Errorf("RED: internal/routingtest coverage = %.1f%%, want >= 90.0%% (baseline 80.7%%; engine/bricks/agent 0%% funcs must be probed)", pct)
	}
}
