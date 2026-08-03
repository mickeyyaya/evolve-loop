//go:build acs

// Package cycle1248 materialises the cycle-1248 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane under inbox item
// tdd-structural-test-reachability-probe:
//
//   - reachability-ambiguous-package-resolution → cover resolvePackage's
//     multi-candidate branch (frozenpins.go:302-323)
//   - reachability-gate-docs                    → document the live frozen-pin
//     gate in the canonical gates table
//
// Why these two and not the headline item. The probe itself already shipped:
// CheckCallSite/BuildImportGraph (cycle-1226), FrozenTestFiles/ExtractFrozenPins/
// CheckFrozenPins (cycle-1238), wired into `evolve phase verify tdd` at
// go/internal/cli/phasecmd/phase_verify.go:142-146 with a permanent regression
// guard. What survives is one untested branch and one undocumented gate.
//
// The untested branch, precisely. resolvePackage's ALIAS path is covered
// (frozenpins_test.go:151-275). Its fallback — base-name matching across the
// import graph, scored by longest common prefix with the pinning package and
// tie-broken lexically — has ZERO coverage today: `grep -n "resolvePackage"
// *_test.go` matches only doc comments. That fallback is the multi-package-name
// collision variant of the very cycle-644 failure class the feature exists to
// catch, and Go map iteration order is randomised, so an untested tie-break is a
// latent nondeterminism.
//
// Predicate strategy — every predicate EXECUTES the system under test (the
// cycle-85 degenerate-predicate ban). 001/005/007 run real `go test`
// invocations, each scoped to ONE named package (never a `./...` sweep — the
// flaky-predicate-shape rule). 002/003/004 assert on the NAMES that appear in
// 001's live `-v` run output, so they are claims about tests that actually
// executed and passed, not about source text. 006 is the sole file-content
// assertion and is a documentation-presence criterion by nature; it is paired
// with 007, which proves the documented gate is live rather than vapour.
//
// Naming contract Builder inherits (restated in test-report.md's handoff): the
// new tests live in package reachabilityprobe (resolvePackage is unexported),
// are named with the prefix `TestResolvePackage`, and their subtest names carry
// the case markers 002/003/004 match on.
package cycle1248

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// resolvePackageRun is the scoped `go test` invocation the ambiguity criteria
// are graded against. One named package, -count=1 (no cache), -run filter — the
// shapes the flaky-predicate lint requires.
const (
	probePkg     = "./internal/reachabilityprobe"
	phasecmdPkg  = "./internal/cli/phasecmd"
	resolveFocus = "TestResolvePackage"
)

// goTest runs `go test` inside the worktree's go module directory and returns
// combined output plus exit code. cmd.Dir is set explicitly rather than relying
// on process cwd, which differs between the main tree, this worktree and every
// fleet lane.
func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	cmd := exec.Command("go", append([]string{"test"}, args...)...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go test %v: %v\n%s", args, err, out)
		}
	}
	return string(out), code
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

// resolveRunOutput caches the one live -v run 001-004 all grade against, so the
// four predicates cost one `go test` invocation rather than four.
var (
	resolveOut  string
	resolveCode int
	resolveDone bool
)

func resolveRun(t *testing.T) (string, int) {
	t.Helper()
	if !resolveDone {
		resolveOut, resolveCode = goTest(t, "-count=1", "-run", resolveFocus, "-v", probePkg)
		resolveDone = true
	}
	return resolveOut, resolveCode
}

// passingNames returns the names of every test and subtest that reported PASS in
// a `go test -v` transcript.
func passingNames(out string) []string {
	var names []string
	re := regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	for _, m := range re.FindAllStringSubmatch(out, -1) {
		names = append(names, m[1])
	}
	return names
}

// requireCase fails unless at least one PASSING test/subtest name matches want.
// Grading on PASS lines (not source text) means a case only counts once it has
// actually run green.
func requireCase(t *testing.T, out string, want *regexp.Regexp, label string) {
	t.Helper()
	for _, name := range passingNames(out) {
		if want.MatchString(name) {
			t.Logf("%s satisfied by passing test %s", label, name)
			return
		}
	}
	t.Errorf("no PASSING test covers %s (want name matching %s)\npassing names: %v\n--- transcript ---\n%s",
		label, want, passingNames(out), out)
}

// TestC1248_001_resolve_package_tests_execute_green is the base criterion: a
// TestResolvePackage* suite exists in the reachabilityprobe package and passes.
// RED today — `-run TestResolvePackage` currently matches nothing, so the run
// emits "no tests to run" and yields zero PASS lines.
func TestC1248_001_resolve_package_tests_execute_green(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s ./internal/reachabilityprobe exited %d\n%s", resolveFocus, code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("no test matches -run %s: resolvePackage's base-name fallback is still uncovered\n%s", resolveFocus, out)
	}
	names := passingNames(out)
	if len(names) == 0 {
		t.Fatalf("expected at least one PASSING %s* test, got none\n%s", resolveFocus, out)
	}
	t.Logf("passing %s* tests/subtests: %v", resolveFocus, names)
}

// TestC1248_002_covers_multi_candidate_selection pins the primary ambiguity
// case: two or more packages in the graph share the pinned identifier's base
// name, and the nearest neighbour (longest shared prefix with the pinning
// package) must win.
func TestC1248_002_covers_multi_candidate_selection(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)ambig|multi|candidate|nearest|prefix`),
		"multi-candidate base-name selection (nearest neighbour wins)")
}

// TestC1248_003_covers_deterministic_tie_break pins the determinism half. Map
// iteration order is randomised, so a tie between equally-scored candidates is
// exactly where a nondeterministic verdict would hide; the case must assert the
// lexically-smallest candidate wins, stably.
func TestC1248_003_covers_deterministic_tie_break(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)tie|determin|lexic|stable`),
		"deterministic lexical tie-break between equally-scored candidates")
}

// TestC1248_004_covers_no_candidate_fail_open is the NEGATIVE axis, and the
// strongest anti-no-op signal here: an identifier NO package in the graph
// matches must resolve to nothing and produce no violation. Without this a
// happy-path-only suite would pass against a resolver that reported a
// best-guess for every input.
func TestC1248_004_covers_no_candidate_fail_open(t *testing.T) {
	out, code := resolveRun(t)
	if code != 0 {
		t.Fatalf("go test -run %s exited %d\n%s", resolveFocus, code, out)
	}
	requireCase(t, out, regexp.MustCompile(`(?i)nomatch|no_?candidate|unknown|unresolv|fail_?open|absent|none`),
		"negative case: identifier matching no package resolves to nothing (fails open)")
}

// TestC1248_005_reachabilityprobe_package_green is the no-regression floor for
// task 1: the whole reachabilityprobe package — not just the new tests — stays
// green. Single named package, per the flaky-predicate-shape rule.
func TestC1248_005_reachabilityprobe_package_green(t *testing.T) {
	out, code := goTest(t, "-count=1", probePkg)
	if code != 0 {
		t.Fatalf("go test %s exited %d\n%s", probePkg, code, out)
	}
}

// TestC1248_006_gates_table_documents_frozen_pin_gate grades task 2: the
// canonical gates table in runtime-reference.md — the file CLAUDE.md tells every
// session to read before touching gate behaviour — must name the frozen-pin
// reachability check, the symbol that implements it, and the live seam it runs
// at. Symbol-anchored so a vague "we check reachability" sentence cannot pass.
//
// acs-predicate: config-check — documentation presence is inherently a
// content criterion; the behavioural half is C1248_007, which proves the
// documented gate actually runs.
func TestC1248_006_gates_table_documents_frozen_pin_gate(t *testing.T) {
	doc := filepath.Join(acsassert.RepoRoot(t), "docs/operations/runtime-reference.md")
	acsassert.FileExists(t, doc)
	for _, anchor := range []string{
		"CheckFrozenPins",
		"reachabilityprobe",
		"evolve phase verify tdd",
	} {
		acsassert.FileContains(t, doc, anchor)
	}
	// The entry must sit in a gates-table row (leading pipe), not a stray
	// paragraph elsewhere in the file.
	acsassert.FileMatchesRegex(t, doc, `(?m)^\|.*(CheckFrozenPins|reachabilityprobe).*\|`)
}

// TestC1248_007_documented_gate_is_live proves the row added by task 2 documents
// a real, running gate: the permanent phasecmd regression guard that drives
// `evolve phase verify tdd` end to end must pass. Pre-existing GREEN by design —
// it is the anti-vapour pairing for the documentation predicate, and it turns RED
// the moment a docs change is accompanied by a broken gate.
func TestC1248_007_documented_gate_is_live(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestPhaseVerifyTDD_FrozenPin", phasecmdPkg)
	if code != 0 {
		t.Fatalf("go test -run TestPhaseVerifyTDD_FrozenPin %s exited %d\n%s", phasecmdPkg, code, out)
	}
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("the frozen-pin gate guard has vanished; the documented gate is not live\n%s", out)
	}
}
