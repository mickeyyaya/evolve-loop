//go:build acs

// Package cycle433 materialises the cycle-433 acceptance criteria for slice
// S5 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S5 is the campaign's final slice: harden SignalCenter's concurrency model
// under ParallelEvaluate-style dispatch and resolve ADR-0068's "Deferred
// (S5)" per-session sharding decision with measured evidence.
//
// Tasks (Task B dependsOn Task A):
//
//	s5-parallelevaluate-stress-race (Task A — S, P0, dependsOn: none):
//	  A mixed-op stress harness (≥16 producers × ≥100 Observe cycles, plus
//	  concurrent Aggregate/Busy/Changed readers and concurrent
//	  RegisterHandler) on ONE shared *SignalCenter, race-clean, asserting the
//	  Aggregate() invariant on every read. Written against the ALREADY-SHIPPED
//	  SignalCenter (S2-S4, on main) — no production change is required, so
//	  these predicates run GREEN today (pre-existing GREEN; they PIN the
//	  concurrency invariant for Task B's evidence-driven refactor).
//
//	s5-resolve-sharding-decision (Task B — M, P0, dependsOn Task A):
//	  Add BenchmarkSignalCenter_ParallelObserve (distinct session keys); use
//	  its measured result to RESOLVE ADR-0068's deferred sharding decision
//	  (implement minimal per-session sharding OR record
//	  single-mutex-sufficient); document the concurrency model (ownership,
//	  lock ordering, no-data-race invariant). RED today: the ADR still
//	  contains the literal "Deferred (S5)" string, has no lock-ordering
//	  phrase, and no BenchmarkSignalCenter_ParallelObserve exists anywhere in
//	  the panestream package.
//
// AC map (1:1 against the full graders in
// .evolve/evals/s5-parallelevaluate-stress-race.md and
// .evolve/evals/s5-resolve-sharding-decision.md; predicates for ## top_n
// tasks only, R9.3 floor-binding):
//
//	Task A (s5-parallelevaluate-stress-race):
//	  AC1 stress test exists, ≥16 producers × ≥100 cycles, mixed overlapping
//	      ops (positive)                                     → C433_001 pre-existing GREEN
//	  AC2 race-clean under -race (regression)                → C433_001 (same run, -race flag) pre-existing GREEN
//	  AC3 real guard: exercises ALL FIVE ops, not a cheap
//	      distinct-keys-only fake (anti-gaming)               → C433_003 pre-existing GREEN
//	  AC4 Aggregate never invalid under concurrency, incl. the
//	      same-key torn-read shape (BA2) (edge/negative)      → C433_001 + C433_002 pre-existing GREEN
//	  AC5 apicover clean, no new exported symbol (regression) → C433_004 pre-existing GREEN
//
//	Task B (s5-resolve-sharding-decision):
//	  AC1 distinct-key Observe benchmark exists and runs      → C433_007 RED today (no such func exists)
//	  AC2 ADR-0068 no longer says "Deferred (S5)" (negative)  → C433_005 RED today
//	  AC3 ADR documents concurrency model: ownership + lock
//	      ordering + no-data-race invariant (positive)        → C433_006 RED today
//	  AC4 IF sharded: Observe on distinct keys no longer holds
//	      a single process-global lock across Assess(); lock
//	      ordering documented+enforced; Aggregate/Busy/Changed
//	      read under the SAME lock Observe writes under (no
//	      torn read) — contingent on a decision not yet made   → manual+checklist (see test-report.md; C433_002/C433_008 are the automated backstop regardless of which branch is chosen)
//	  AC5 public API unchanged; no new exported symbol without
//	      an AST-naming test (regression)                     → C433_004 + C433_008 pre-existing GREEN
//
// RED strategy: C433_001-004 are pre-existing GREEN (Task A needs zero
// production change — the current single-RWMutex model, S2-S4, already
// satisfies it). C433_005-007 are genuinely RED today, confirmed by direct
// rerun immediately before test-report.md was written. C433_008 (full-suite
// regression) is pre-existing GREEN and MUST stay green after Task B lands.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C433_002 (a torn read on the SAME key, not merely disjoint
//	            keys — the shape that would surface BA2), C433_003 (a test
//	            that only drives Observe on distinct keys — the cheapest
//	            possible fake — FAILS this grader; forces genuine op
//	            diversity across all five SignalCenter operations)
//	Edge/OOD:   C433_001 re-runs the pre-existing
//	            TestSignalCenter_EmptyCenter_DefinedState /
//	            TestSignalCenter_UnknownKeyIsQuiet under the SAME -race
//	            harness invocation (empty-center / unknown-key reads must
//	            stay defined under concurrency, not merely standalone)
//	Semantic:   C433_005 (doc text: deferral removed) vs. C433_006 (doc text:
//	            model documented) are DISTINCT textual claims, not one
//	            assertion restated — an ADR edit that removes "Deferred (S5)"
//	            without adding the lock-ordering/ownership documentation
//	            passes C433_005 but still fails C433_006
//
// 1:1 enforcement: Task A predicate=5 (AC1-5) → total=5 ✓ (matches the eval
// file's 5-AC list). Task B predicate=4 (AC1,AC2,AC3,AC5) + manual+checklist=1
// (AC4) → total=5 ✓ (matches the eval file's 5-AC list). Combined: 10/10 ACs
// dispositioned, zero bare "defer to Auditor" entries.
package cycle433

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	panestreamImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	bridgeImportPath     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	adrRelPath           = "docs/architecture/adr/0068-bridge-signal-center-concurrency.md"
)

// runRace shells to `go test -race` (optionally -run filtered) over the given
// import paths. Mirrors go/acs/cycle431/predicates_test.go's runRaceSuite —
// the established two-layer pattern for this campaign (ACS predicate wraps a
// subprocess `go test` invocation of the real, underlying white-box tests).
func runRace(t *testing.T, runFilter string, pkgs ...string) (string, string, int) {
	t.Helper()
	args := []string{"test", "-race", "-count=1"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

func adrPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), adrRelPath)
}

// ── Task A: s5-parallelevaluate-stress-race ─────────────────────────────────

// TestC433_001_ParallelEvaluateStressRaceClean (AC1/AC2/AC4, positive +
// regression, pre-existing GREEN): the ≥16-producer/≥100-cycle mixed-op
// stress harness, plus the pre-existing empty-center/unknown-key edge tests,
// all race-clean together.
func TestC433_001_ParallelEvaluateStressRaceClean(t *testing.T) {
	runFilter := "TestSignalCenter_ParallelEvaluateStress_MixedOpsRaceClean|" +
		"TestSignalCenter_EmptyCenter_DefinedState|TestSignalCenter_UnknownKeyIsQuiet"
	stdout, stderr, code := runRace(t, runFilter, panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_001: mixed-op stress test exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

// TestC433_002_SameKeyObserveAggregateRaceClean (AC4/BA2, negative/edge,
// pre-existing GREEN): concurrent Observe vs. Aggregate/Busy/Changed on the
// SAME session key — the torn-read shape (BA2) a naive disjoint-keys-only
// test would never exercise.
func TestC433_002_SameKeyObserveAggregateRaceClean(t *testing.T) {
	_, stderr, code := runRace(t, "TestSignalCenter_ObserveAggregateSameKeyRaceClean", panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_002: same-key race test (BA2 guard) exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC433_003_StressTestExercisesAllFiveOps (AC3, anti-gaming, pre-existing
// GREEN): the stress test file must call all five SignalCenter operations —
// a test that only drives Observe on distinct keys (the cheapest possible
// fake) fails this grader. Checks the TEST FILE's own completeness (not
// production source), mirroring the eval file's COVERS_ALL_OPS grader
// exactly.
func TestC433_003_StressTestExercisesAllFiveOps(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "bridge", "panestream",
		"signalcenter_parallelevaluate_test.go")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("C433_003: read %s: %v", path, err)
	}
	src := string(b)
	for _, op := range []string{"NewSignalCenter(", ".Observe(", ".Aggregate(", ".Busy(", ".Changed(", "RegisterHandler("} {
		if !strings.Contains(src, op) {
			t.Errorf("C433_003: signalcenter_parallelevaluate_test.go missing a call to %q — the stress test must exercise ALL FIVE SignalCenter ops on a shared center, not a cheap fake that only drives Observe on distinct keys", op)
		}
	}
}

// TestC433_004_ApicoverEnforceCleanBothPackages (AC5, regression,
// pre-existing GREEN): apicover -enforce must report 0 uncovered / 0
// false-green symbols on both touched packages — the recurring
// cycles-413/426/430 CI-break class. Mirrors
// go/acs/cycle431/predicates_test.go's TestC431_004 exactly.
func TestC433_004_ApicoverEnforceCleanBothPackages(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover433")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C433_004: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage433.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C433_004: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C433_004: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage433.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C433_004: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C433_004: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C433_004: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C433_004: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

// ── Task B: s5-resolve-sharding-decision ────────────────────────────────────

// TestC433_005_ADRNoLongerDeferred (AC2, negative, RED today): ADR-0068 must
// no longer contain the literal "Deferred (S5)" string. FileNotContains
// (not an inverted FileContains — see go/acs/README.md's cycle-352 lesson)
// passes silently when the string is absent and fails when it is present.
func TestC433_005_ADRNoLongerDeferred(t *testing.T) {
	acsassert.FileNotContains(t, adrPath(t), "Deferred (S5)")
}

// TestC433_006_ADRDocumentsConcurrencyModel (AC3, positive, RED today):
// ADR-0068 must document BOTH the lock-ordering rule AND ownership/mutation
// semantics — two distinct textual claims (see Semantic diversity note
// above), not one assertion restated.
func TestC433_006_ADRDocumentsConcurrencyModel(t *testing.T) {
	path := adrPath(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("C433_006: read ADR: %v", err)
	}
	lower := strings.ToLower(string(b))

	hasLockOrder := strings.Contains(lower, "lock order") || strings.Contains(lower, "acquisition order")
	if !hasLockOrder {
		t.Errorf("C433_006: ADR-0068 missing lock-ordering documentation (want 'lock order'/'lock ordering'/'acquisition order')")
	}

	hasOwnership := strings.Contains(lower, "ownership") || strings.Contains(lower, "mutat")
	if !hasOwnership {
		t.Errorf("C433_006: ADR-0068 missing ownership/mutation documentation (want 'ownership' or 'mutat(e/ion)')")
	}
}

// TestC433_007_ContentionBenchmarkExistsAndRuns (AC1, positive, RED today):
// BenchmarkSignalCenter_ParallelObserve must exist and run — this actually
// invokes `go test -bench` (a real subprocess execution of the SUT), not a
// source grep, so it is exempt from the FileContains-over-source ban (it
// exercises the benchmark, it doesn't just check the file for magic text).
func TestC433_007_ContentionBenchmarkExistsAndRuns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-run", "^$", "-bench", "SignalCenter_ParallelObserve",
		"-benchtime=10x", "./internal/bridge/panestream/...",
	)
	if code != 0 {
		t.Errorf("C433_007: benchmark run exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
		return
	}
	if !strings.Contains(stdout, "BenchmarkSignalCenter_ParallelObserve") {
		t.Errorf("C433_007: no BenchmarkSignalCenter_ParallelObserve in output — benchmark does not exist yet\nstdout=%s", stdout)
	}
}

// TestC433_008_FullRaceSuiteGreen (AC5, regression, pre-existing GREEN): the
// full bridge + panestream -race suite (every wedge invariant: cycles
// 254/255, 262, 274/277, 286/288, 291/311/312) must stay green — the
// regression backstop for Task B's refactor, whichever branch is chosen.
func TestC433_008_FullRaceSuiteGreen(t *testing.T) {
	_, stderr, code := runRace(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C433_008: full bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}
