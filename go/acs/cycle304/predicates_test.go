//go:build acs

// Package cycle304 materializes the cycle-304 acceptance criteria for the single
// committed top_n task (triage-report.md ## top_n — blocker-solo rule, ADR-0046
// Core Principle 5):
//
//	T1  declarative-floor-counter — replace prose-regex floor counting in
//	    internal/triagecap with DECLARATION-primary counting sourced from the
//	    triage-decision.json companion's committed_floors[] array, retaining the
//	    prose counter only as fallback. Closes the phantom-floor class that failed
//	    cycles 301 and 302 (the bullet contract's mandated evidence=/source=scout
//	    tokens and coverage prose collided with real package basenames, inflating
//	    the floor count and making the capacity-clamp correction unsatisfiable).
//
// These predicates are BEHAVIORAL (cycle-85 lesson; cycle281/300 pattern). The
// load-bearing gate RUNS the real internal/triagecap test suite as a subprocess
// (`go test -v -run <the five TDD pins> ./internal/triagecap/`) and asserts on the
// real `--- PASS: <name>` lines the builder's implementation produces. Those five
// tests construct companions, run the real readers, and run the real CapReviewer /
// Recorder against them — a magic string in a .go file can neither produce a named
// PASS line nor make a declaration-primary count match, and an EMPTY repo lacks
// the functions entirely (compile failure → no PASS lines → RED). The config-check
// predicate (schema + persona declare committed_floors) carries an explicit waiver:
// it is an inherent presence check on the agent-facing contract surface, auxiliary
// to the behavioral gate above.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", declarative-
// floor-counter rows):
//
//	C1 Declaration count exact          TestCountFromDeclaration        \
//	C4 Missing companion -> prose       TestCountFallbackToProse         |
//	C3 Divergence -> satisfiable corr.  TestFloorDivergenceCorrective    } C304_001
//	C6 Reviewer uses declared count     TestReviewer_UsesDeclaredFloors  |
//	C7 Recorder uses declared count     TestRecorder_DeclaredFloors     /
//	(contract surface) schema + persona declare committed_floors        -> C304_002
//
// Floor binding (R9.3): declarative-floor-counter is NOT a coverage-floor task —
// it commits zero package coverage floors this cycle, so no coverage-floor
// predicate is authored. The triage-deferred items (evalgate-floor-declarations,
// the Layer 2/3 work, ledger-1740) get ZERO predicates here.
package cycle304

import (
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// The five TDD pins that encode the declarative-floor-counter behavior, run as
// one scoped subprocess so an unrelated triagecap regression cannot false-RED
// this gate. These tests live in
// go/internal/triagecap/declarative_floors_test.go (authored by the TDD engineer)
// and the builder turns them GREEN.
var declPins = []string{
	"TestCountFromDeclaration",
	"TestCountFallbackToProse",
	"TestFloorDivergenceCorrective",
	"TestReviewer_UsesDeclaredFloors",
	"TestRecorder_DeclaredFloors",
}

var (
	triageOnce sync.Once
	triageOut  string
)

// runTriagePins runs ONLY the five declarative-floor-counter pins, verbose, once
// per predicate process.
func runTriagePins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	triageOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(declPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./internal/triagecap/")
		triageOut = stdout + "\n" + stderr
	})
	return triageOut
}

func regexpAlternation(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "|"
		}
		out += regexp.QuoteMeta(n)
	}
	return out
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	noTestsRe  = regexp.MustCompile(`(?m)^testing: warning: no tests to run|no test files`)
)

// topLevelPassed reports whether a `--- PASS: <name>` line names exactly `name`
// (top-level test, not a subtest path like Parent/sub).
func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

// --- C304_001 (T1): the five declarative-floor-counter pins exist and PASS -----
//
// Behavioral gate. Each pin RUNS the real system: ReadDeclaredFloors over a real
// companion file, CommittedFloorCount's declaration-primary-with-prose-fallback,
// FloorDivergenceCorrective's cross-examiner, and the real CapReviewer / Recorder
// against a workspace+companion. RED baseline: ReadDeclaredFloors /
// CommittedFloorCount / FloorDivergenceCorrective do not exist, so
// internal/triagecap fails to compile → zero PASS lines → this predicate fails.
func TestC304_001_DeclarativeFloorCounterPinsPass(t *testing.T) {
	out := runTriagePins(t)
	if noTestsRe.MatchString(out) {
		t.Fatalf("RED: the five declarative-floor-counter pins did not run (not authored / not compiling):\n%s", tail(out, 40))
	}
	if anyFailRe.MatchString(out) {
		t.Fatalf("RED: a declarative-floor-counter pin FAILED — make the declaration-primary path GREEN:\n%s", tail(out, 60))
	}
	for _, name := range declPins {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: pin %s did not emit `--- PASS` (missing/failed/renamed):\n%s", name, tail(out, 60))
		}
	}
}

// --- C304_002 (contract surface): schema + persona declare committed_floors ----
//
// acs-predicate: config-check — WAIVED. The committed_floors field is an agent-
// facing contract surface: the triage persona must instruct emitting it and the
// handoff schema must document it, or the declaration readers gate on data the
// agent never writes (the readers would be dead code, silently reverting every
// cycle to the prose path this task replaces). This is an inherent presence check
// with no behavioral subprocess to run; the behavioral weight is carried by
// C304_001.
func TestC304_002_SchemaAndPersonaDeclareCommittedFloors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	schema := filepath.Join(root, "schemas", "handoff", "triage-decision.schema.json")
	persona := filepath.Join(root, "agents", "evolve-triage.md")

	if !acsassert.FileExists(t, schema) {
		t.Fatalf("RED: %s missing", schema)
	}
	if !acsassert.FileContains(t, schema, "committed_floors") {
		t.Errorf("RED: triage-decision schema does not document committed_floors — the declaration field is ungoverned")
	}
	if !acsassert.FileExists(t, persona) {
		t.Fatalf("RED: %s missing", persona)
	}
	if !acsassert.FileContains(t, persona, "committed_floors") {
		t.Errorf("RED: triage persona does not instruct emitting committed_floors — readers would gate on data the agent never writes")
	}
}

// tail returns the last n lines of s (subprocess output is long; keep failures
// readable).
func tail(s string, n int) string {
	lines := splitLines(s)
	if len(lines) <= n {
		return s
	}
	out := ""
	for _, l := range lines[len(lines)-n:] {
		out += l + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
