//go:build acs

// Package cycle1241 materialises the cycle-1241 acceptance criteria for the
// fleet-scoped todo `tdd-structural-test-reachability-probe`, committed by
// triage as two tasks:
//
//   - acs-current-cycle-scope-reachability-probe — a new
//     `go/internal/acssuite/reachability_test.go` whose
//     `TestCurrentCycleScopeReachable` proves the current-cycle ACS scope is
//     reachable END TO END: `goLanePatterns(moduleDir, cycle)` selects it AND a
//     real `go test` against that pattern produces real test events. It closes
//     the gap between `TestAllACSPredicatesAreTagged` (tagging only) and the
//     existing `TestGoLanePatterns_*` unit cases (pattern strings in isolation).
//   - acs-scope-cyclenum-drift-adversarial-case — a new
//     `TestGoLanePatterns_CycleNumberDrift` in
//     `go/internal/acssuite/acssuite_adversarial_test.go` asserting that a
//     cycle-number/dir-name mismatch EXCLUDES the scope rather than silently
//     matching it loosely.
//
// Provenance: cycle-1222 was a prior attempt at this same todo. It left a RED
// predicate suite (`go/acs/cycle1222/predicates_test.go`) and an EGPS failure
// recorded at `.evolve/runs/cycle-1222/audit-fail-reason.json`
// (`red_count=3`). That file is the authoritative acceptance spec and must go
// GREEN **unmodified** — predicate 005 below is what proves that, because the
// ACS gate runs only `./acs/cycle1241` this cycle and would never otherwise
// execute the cycle-1222 scope.
//
// Predicate strategy — every predicate drives the REAL `go` toolchain against
// the REAL package and asserts on observed `go test -v` events. None asserts
// that a string is present in a source file (the cycle-85 degenerate-predicate
// ban): a no-op could satisfy that by pasting a func name into a comment.
//
//   - 001 the reachability guard must RUN and PASS.
//   - 002 the drift guard must RUN and PASS.
//   - 003 NEGATIVE / anti-vacuity control: a test name that must never exist
//     has to produce "no tests to run" and zero PASS events. If 003 is RED the
//     PASS-line assertions in 001/002/004/005 are not evidence of anything.
//   - 004 WIRING proof: both guards must be reached by the plain, untagged
//     `go test ./internal/acssuite` that CI actually runs — no `-tags`, no
//     `-run`. A guard that inherits `//go:build acs` from the surrounding ACS
//     convention would pass 001/002 under an explicit `-run` while being
//     invisible to every CI run. A guard nothing reaches is dead code.
//   - 005 the cycle-1222 predicate suite goes fully GREEN under `-tags acs`,
//     with its own file unmodified (asserted via `git status --porcelain`).
package cycle1241

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// reachabilityTestName is the guard the reachability-probe task must add.
const reachabilityTestName = "TestCurrentCycleScopeReachable"

// driftTestName is the guard the cycle-number-drift task must add.
const driftTestName = "TestGoLanePatterns_CycleNumberDrift"

// acssuitePkg is the single named package under test. Scoped deliberately:
// never a `./...` sweep, never one of the known-slow suites (./internal/core,
// ./cmd/evolve).
const acssuitePkg = "./internal/acssuite"

// priorPredicatePkg is the cycle-1222 predicate scope that must go GREEN.
const priorPredicatePkg = "./acs/cycle1222"

// priorPredicateRel is the same file, repo-relative, for the unmodified check.
const priorPredicateRel = "go/acs/cycle1222/predicates_test.go"

// goModuleDir resolves <tree>/go from THIS predicate file's own location
// (<tree>/go/acs/cycle1241/predicates_test.go → ../..). Deriving it from the
// file rather than the process cwd means the predicate always exercises the
// tree that HOLDS it — main tree, cycle worktree, or fleet lane alike.
func goModuleDir(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed: cannot locate this predicate file")
	}
	dir, err := filepath.Abs(filepath.Join(filepath.Dir(self), "..", ".."))
	if err != nil {
		t.Fatalf("resolve go module dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("resolved module dir %s has no go.mod: %v", dir, err)
	}
	return dir
}

// goTestRun executes `go test -count=1 -v [-tags acs] [-run pattern] <pkg>` in
// the module dir and returns the combined output plus the exit code. The
// context is a HANG guard only — no assertion is derived from elapsed time.
// cmd.Dir — never the process cwd — pins the tree under test.
func goTestRun(t *testing.T, moduleDir, pkg, runPattern string, tags bool) (out string, code int) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go toolchain not on PATH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	args := []string{"test", "-count=1", "-v"}
	if tags {
		args = append(args, "-tags", "acs")
	}
	if runPattern != "" {
		args = append(args, "-run", runPattern)
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = os.Environ()
	cmd.WaitDelay = 10 * time.Second
	raw, err := cmd.CombinedOutput()
	out = string(raw)
	code = 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running `go %s` in %s: %v\noutput:\n%s", strings.Join(args, " "), moduleDir, err, out)
		}
	}
	return out, code
}

// passLine is the `go test -v` event a named test emits when it runs AND
// passes. Absence means the test did not run (missing / renamed / excluded by a
// build tag) or it failed — both are RED for these criteria.
func passLine(name string) string { return "--- PASS: " + name }

// TestC1241_001_ReachabilityProbeGuardRunsAndPasses drives the real toolchain
// against the real package and requires `TestCurrentCycleScopeReachable` to
// actually execute and pass.
//
// RED today: the guard does not exist, so `go test -run` emits
// "testing: warning: no tests to run" and never emits the PASS event.
func TestC1241_001_ReachabilityProbeGuardRunsAndPasses(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, acssuitePkg, "^"+reachabilityTestName+"$", false)

	if strings.Contains(out, "no tests to run") {
		t.Errorf("C1241-001: `go test -run ^%s$ %s` matched NOTHING — the reachability guard "+
			"(task acs-current-cycle-scope-reachability-probe) is absent from the package.\noutput:\n%s",
			reachabilityTestName, acssuitePkg, excerpt(out))
	}
	if !strings.Contains(out, passLine(reachabilityTestName)) {
		t.Errorf("C1241-001: expected event %q in `go test -v` output; the guard must RUN and PASS "+
			"(exit=%d).\noutput:\n%s", passLine(reachabilityTestName), code, excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1241-001: `go test -run ^%s$ %s` exited %d, want 0.\noutput:\n%s",
			reachabilityTestName, acssuitePkg, code, excerpt(out))
	}
}

// TestC1241_002_CycleNumberDriftGuardRunsAndPasses requires the adversarial
// drift case to actually execute and pass.
//
// RED today: `TestGoLanePatterns_CycleNumberDrift` does not exist.
func TestC1241_002_CycleNumberDriftGuardRunsAndPasses(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, acssuitePkg, "^"+driftTestName+"$", false)

	if strings.Contains(out, "no tests to run") {
		t.Errorf("C1241-002: `go test -run ^%s$ %s` matched NOTHING — the drift adversarial case "+
			"(task acs-scope-cyclenum-drift-adversarial-case) is absent.\noutput:\n%s",
			driftTestName, acssuitePkg, excerpt(out))
	}
	if !strings.Contains(out, passLine(driftTestName)) {
		t.Errorf("C1241-002: expected event %q in `go test -v` output; the guard must RUN and PASS "+
			"(exit=%d).\noutput:\n%s", passLine(driftTestName), code, excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1241-002: `go test -run ^%s$ %s` exited %d, want 0.\noutput:\n%s",
			driftTestName, acssuitePkg, code, excerpt(out))
	}
}

// TestC1241_003_HarnessRejectsAbsentGuard is the NEGATIVE / anti-vacuity
// control for this predicate file: a test name that must never exist has to
// produce the "no tests to run" warning and zero PASS events.
//
// If this predicate ever goes RED, the PASS-line assertions in 001/002/004/005
// are not evidence of anything — `go test -run` would be matching (or
// reporting) something other than what those predicates believe. It is
// deliberately independent of whether the two deliverables landed.
func TestC1241_003_HarnessRejectsAbsentGuard(t *testing.T) {
	moduleDir := goModuleDir(t)
	const sentinel = "TestC1241SentinelThatMustNeverExist"

	out, code := goTestRun(t, moduleDir, acssuitePkg, "^"+sentinel+"$", false)

	if !strings.Contains(out, "no tests to run") {
		t.Errorf("C1241-003: harness unsound — `go test -run ^%s$` on a nonexistent test did NOT warn "+
			"\"no tests to run\"; the PASS-line assertions in C1241-001/002/004/005 cannot be trusted.\noutput:\n%s",
			sentinel, excerpt(out))
	}
	if strings.Contains(out, "--- PASS: "+sentinel) {
		t.Errorf("C1241-003: harness unsound — a nonexistent test reported a PASS event.\noutput:\n%s", excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1241-003: `go test -run ^%s$ %s` exited %d; the package itself must build and the "+
			"empty selection must be a clean exit.\noutput:\n%s", sentinel, acssuitePkg, code, excerpt(out))
	}
}

// TestC1241_004_GuardsRunInDefaultUntaggedSuite is the WIRING/REACHABILITY
// proof: both new guards must be reached by the plain, untagged
// `go test ./internal/acssuite` that CI runs — no `-tags acs`, no `-run`
// selector.
//
// This is the load-bearing predicate for the class the todo names. A new guard
// authored in this neighbourhood can very plausibly inherit `//go:build acs`
// from the surrounding ACS convention (or land in a file excluded some other
// way): it would then pass under an explicit `-run` invocation while being
// invisible to every CI run. A guard nothing reaches is dead code. The same run
// also proves the additions leave the whole package green (no regression).
func TestC1241_004_GuardsRunInDefaultUntaggedSuite(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, acssuitePkg, "", false)

	for _, name := range []string{reachabilityTestName, driftTestName} {
		if !strings.Contains(out, passLine(name)) {
			t.Errorf("C1241-004: %q did not run+pass in the DEFAULT untagged suite "+
				"(`go test -count=1 -v %s`, no -tags and no -run). A guard CI never executes is dead "+
				"code — check it is not behind `//go:build acs` or an excluded file.\noutput:\n%s",
				name, acssuitePkg, excerpt(out))
		}
	}
	if code != 0 {
		t.Errorf("C1241-004: the full `%s` package suite exited %d, want 0 — the new guards must not "+
			"regress the package.\noutput:\n%s", acssuitePkg, code, excerpt(out))
	}
}

// TestC1241_005_PriorCycle1222PredicatesGoGreenUnmodified closes the cycle's
// stated acceptance bar: the surviving cycle-1222 predicate suite — the
// authoritative spec for this todo, which recorded red_count=3 — must go fully
// GREEN, and it must do so WITHOUT being edited (editing the spec to match the
// implementation is the classic way to fake this).
//
// The ACS gate runs only `./acs/cycle1241` this cycle, so nothing else executes
// the cycle-1222 scope; without this predicate the "cycle-1222 goes GREEN"
// criterion would ship unverified.
func TestC1241_005_PriorCycle1222PredicatesGoGreenUnmodified(t *testing.T) {
	moduleDir := goModuleDir(t)

	if _, err := os.Stat(filepath.Join(moduleDir, "acs", "cycle1222", "predicates_test.go")); err != nil {
		t.Fatalf("C1241-005: the cycle-1222 predicate spec is missing from this tree (%v) — it is the "+
			"acceptance spec for this todo and must NOT be deleted", err)
	}

	// Unmodified: git must report no working-tree change for that path. `git -C`
	// pins the repo; a bare `git` would resolve from the process cwd, which
	// differs between main tree, worktree, and each fleet lane.
	repoRoot := filepath.Dir(moduleDir)
	st := gitPorcelain(t, repoRoot, priorPredicateRel)
	if st != "" {
		t.Errorf("C1241-005: %s has working-tree changes (`git status --porcelain` → %q). The cycle-1222 "+
			"predicate file is the acceptance SPEC and must go green unmodified — do not edit the spec to "+
			"match the implementation.", priorPredicateRel, st)
	}

	out, code := goTestRun(t, moduleDir, priorPredicatePkg, "", true)
	if code != 0 {
		t.Errorf("C1241-005: `go test -tags acs %s` exited %d, want 0 — the cycle-1222 predicate suite "+
			"(red_count=3 at that cycle) must be fully GREEN.\noutput:\n%s", priorPredicatePkg, code, excerpt(out))
	}
	for _, name := range []string{
		"TestC1222_001_ReachabilityProbeGuardRunsAndPasses",
		"TestC1222_002_CycleNumberDriftGuardRunsAndPasses",
		"TestC1222_003_HarnessRejectsAbsentGuard",
		"TestC1222_004_GuardsRunInDefaultUntaggedSuite",
	} {
		if !strings.Contains(out, passLine(name)) {
			t.Errorf("C1241-005: expected event %q from `go test -tags acs -v %s`; the prior cycle's "+
				"predicate must RUN and PASS.\noutput:\n%s", passLine(name), priorPredicatePkg, excerpt(out))
		}
	}
}

// gitPorcelain returns the `git status --porcelain` line for one path, or "".
func gitPorcelain(t *testing.T, repoRoot, rel string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git not on PATH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain", "--", rel)
	cmd.Env = os.Environ()
	cmd.WaitDelay = 5 * time.Second
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status --porcelain -- %s in %s: %v\noutput:\n%s", rel, repoRoot, err, string(raw))
	}
	return strings.TrimSpace(string(raw))
}

// excerpt bounds pasted subprocess output so a failure message stays readable.
func excerpt(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}
