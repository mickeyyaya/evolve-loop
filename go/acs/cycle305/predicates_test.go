//go:build acs

// Package cycle305 materializes the cycle-305 acceptance criteria for the single
// committed-AND-buildable task in this slice (triage-report.md ## top_n, narrowed
// by architecture-design.md to Option A — Layer 1 only):
//
//	evalgate-floor-declarations — complete ADR-0046 Layer 1 by making deferred
//	    floor lookup DECLARATION-primary. Add triagecap.ReadDeferredFloors +
//	    DeferredFloorPackagesDecl (companion deferred_floors[] authoritative, prose
//	    fallback) + DeferredFloorDivergence (the guard's reporter); rewire
//	    evalgate's floorBindingGate to read <workspace>/triage-decision.json; add
//	    deferred_floors to the schema + triage persona; add the
//	    `evolve guard triage-floors <workspace>` self-check CLI. Closes the last
//	    prose-scrape path that left the cycle-280 binding class open.
//
// The second top_n task (heuristic-gate-demotion-instinct, ADR-0046 Layer 2) is
// EXPLICITLY DEFERRED by the architecture-design phase (Option A; design.md:92,
// 152-156): "do not implement GateClass, HeuristicDemotionChecker, inbox demotion
// filing, or orchestrator demotion wiring in this Builder slice." It therefore
// gets ZERO predicates here — authoring demotion pins would gate work the Builder
// is instructed not to do, RED-locking the cycle (the cycle-280 starvation class
// in spirit). Its H1-H4 ACs carry to the next cycle with their own TDD pins.
//
// These predicates are BEHAVIORAL (cycle-85 lesson; cycle281/300/304 pattern). The
// load-bearing gates RUN the real internal/triagecap, internal/evalgate, and
// cmd/evolve test suites as subprocesses and assert on the real `--- PASS: <name>`
// lines the builder's implementation produces. Those pins construct companions,
// run the real readers + the real floorBindingGate + the real guard CLI — a magic
// string in a .go file can neither emit a named PASS line nor make a
// declaration-primary block decision, and an EMPTY tree lacks the functions
// entirely (compile failure → no PASS lines → RED). The config-check predicate
// (schema + persona declare deferred_floors) carries an explicit waiver.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary", evalgate-floor-
// declarations rows C1-C5, plus the triagecap reader pins from the design blueprint):
//
//	C1 companion blocks predicate   TestFloorBinding_DeferredFromCompanion        \
//	C2 missing companion fail-open  TestFloorBinding_MissingCompanion_FailOpen     |
//	N1 prose ignored w/ companion   TestFloorBinding_ProseIgnoredWithCompanion     } C305_001
//	E1 no-field -> prose fallback   TestFloorBinding_CompanionNoField_FallbackProse|
//	C5 divergence reporter          TestFloorBinding_DeclaredDivergenceMessage     |
//	   reader contract              TestReadDeferredFloors / *PackagesDecl* / *Divergence
//	C4 guard CLI self-check         TestGuardTriageFloors_*                         -> C305_002
//	   contract surface             schema + persona declare deferred_floors        -> C305_003
//
// Floor binding (R9.3): evalgate-floor-declarations commits ZERO package coverage
// floors, so no coverage-floor predicate is authored here. All package-path string
// literals below live in non-Test helpers (runDataPins/runGuardPins) so the
// floorBindingGate's Test-function scan never misreads these behavioral gates as
// coverage-floor predicates (cycle304 pattern).
package cycle305

import (
	"path/filepath"
	"regexp"
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

// dataPins are the triagecap + evalgate TDD pins for the declaration-primary
// deferred-floor path. They live in
// go/internal/triagecap/deferred_test.go and go/internal/evalgate/floorbinding_test.go
// (authored by the TDD engineer); the builder turns them GREEN.
var dataPins = []string{
	"TestReadDeferredFloors",
	"TestDeferredFloorPackagesDecl_DeclarationPrimary",
	"TestDeferredFloorPackagesDecl_FallbackToProse",
	"TestDeferredFloorPackagesDecl_CompanionNoFieldFallsBack",
	"TestDeferredFloorPackagesDecl_FiltersToCandidates",
	"TestDeferredFloorDivergence",
	"TestFloorBinding_DeferredFromCompanion",
	"TestFloorBinding_MissingCompanion_FailOpen",
	"TestFloorBinding_ProseIgnoredWithCompanion",
	"TestFloorBinding_CompanionNoField_FallbackProse",
	"TestFloorBinding_DeclaredDivergenceMessage",
}

// guardPins are the cmd/evolve `triage-floors` self-check CLI pins, in
// go/cmd/evolve/cmd_guard_triage_floors_test.go.
var guardPins = []string{
	"TestGuardTriageFloors_CleanWorkspaceExitsZero",
	"TestGuardTriageFloors_DivergenceExitsNonZero",
	"TestGuardTriageFloors_HelpIsInformative",
}

var (
	dataOnce  sync.Once
	dataOut   string
	guardOnce sync.Once
	guardOut  string
)

// runDataPins runs ONLY the triagecap + evalgate deferred-floor pins, verbose,
// once per predicate process. Both packages contain matches, so neither emits a
// "no tests to run" warning unless it fails to compile (the RED baseline).
func runDataPins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	dataOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(dataPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./internal/triagecap/", "./internal/evalgate/")
		dataOut = stdout + "\n" + stderr
	})
	return dataOut
}

// runGuardPins runs ONLY the triage-floors guard CLI pins, verbose, once per
// predicate process.
func runGuardPins(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	guardOnce.Do(func() {
		runExpr := "^(" + regexpAlternation(guardPins) + ")$"
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", runExpr, "./cmd/evolve/")
		guardOut = stdout + "\n" + stderr
	})
	return guardOut
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

func assertAllPass(t *testing.T, out string, pins []string, label string) {
	t.Helper()
	if noTestsRe.MatchString(out) {
		t.Fatalf("RED: the %s pins did not run (not authored / not compiling):\n%s", label, tail(out, 40))
	}
	if anyFailRe.MatchString(out) {
		t.Fatalf("RED: a %s pin FAILED — make the declaration-primary path GREEN:\n%s", label, tail(out, 80))
	}
	for _, name := range pins {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: pin %s did not emit `--- PASS` (missing/failed/renamed):\n%s", name, tail(out, 80))
		}
	}
}

// --- C305_001 (evalgate-floor-declarations, data layer): the declaration-primary
// deferred-floor pins exist and PASS ----------------------------------------------
//
// Behavioral gate. Each pin RUNS the real system: ReadDeferredFloors over a real
// companion, DeferredFloorPackagesDecl's declaration-primary-with-prose-fallback,
// DeferredFloorDivergence's reporter, and the real floorBindingGate.check() against
// a workspace+companion. RED baseline: ReadDeferredFloors / DeferredFloorPackagesDecl
// / DeferredFloorDivergence do not exist, so internal/triagecap AND internal/evalgate
// fail to compile → zero PASS lines → this predicate fails.
func TestC305_001_DeferredDeclarationPinsPass(t *testing.T) {
	assertAllPass(t, runDataPins(t), dataPins, "triagecap+evalgate deferred-declaration")
}

// --- C305_002 (C4): the `evolve guard triage-floors` self-check CLI exists and
// behaves (clean→0, divergence→non-zero, --help informative) --------------------
//
// Behavioral gate. The pins drive the real runGuard() with a constructed
// workspace + companion. RED baseline: `triage-floors` is not a recognized
// subcommand → runGuard falls through to buildGuard → exit 10 → the three pins
// FAIL → no PASS lines → this predicate fails.
func TestC305_002_TriageFloorsGuardCLIWorks(t *testing.T) {
	assertAllPass(t, runGuardPins(t), guardPins, "triage-floors guard CLI")
}

// --- C305_003 (contract surface): schema + persona declare deferred_floors ------
//
// acs-predicate: config-check — WAIVED. deferred_floors is an agent-facing contract
// surface: the triage persona must instruct emitting it and the handoff schema must
// document it, or the declaration reader gates on data the agent never writes (the
// reader silently reverts every cycle to the prose path this task replaces). This is
// an inherent presence check with no behavioral subprocess to run; the behavioral
// weight is carried by C305_001 and C305_002.
func TestC305_003_SchemaAndPersonaDeclareDeferredFloors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	schema := filepath.Join(root, "schemas", "handoff", "triage-decision.schema.json")
	persona := filepath.Join(root, "agents", "evolve-triage.md")

	if !acsassert.FileExists(t, schema) {
		t.Fatalf("RED: %s missing", schema)
	}
	if !acsassert.FileContains(t, schema, "deferred_floors") {
		t.Errorf("RED: triage-decision schema does not document deferred_floors — the declaration field is ungoverned")
	}
	if !acsassert.FileExists(t, persona) {
		t.Fatalf("RED: %s missing", persona)
	}
	if !acsassert.FileContains(t, persona, "deferred_floors") {
		t.Errorf("RED: triage persona does not instruct emitting deferred_floors — the reader would gate on data the agent never writes")
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
