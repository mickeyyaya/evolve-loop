//go:build acs

// Package cycle1222 encodes the cycle-1222 ACS predicates for the fleet-scoped
// todo `tdd-structural-test-reachability-probe`, materialised by triage as two
// committed tasks:
//
//   - acs-current-cycle-scope-reachability-probe — a new
//     `go/internal/acssuite/reachability_test.go` whose
//     `TestCurrentCycleScopeReachable` proves the current-cycle ACS scope is
//     reachable END TO END (goLanePatterns selects it AND `go test -tags acs`
//     against it produces real test events), closing the gap between
//     `TestAllACSPredicatesAreTagged` (tagging only) and `TestGoLanePatterns_*`
//     (pattern logic in isolation).
//   - acs-scope-cyclenum-drift-adversarial-case — a new
//     `TestGoLanePatterns_CycleNumberDrift` in
//     `go/internal/acssuite/acssuite_adversarial_test.go` asserting a
//     cycle-number/dir-name mismatch EXCLUDES the scope rather than silently
//     matching it loosely.
//
// Both deliverables are tests, so every predicate here drives the real `go`
// toolchain against the real `go/internal/acssuite` package and asserts on the
// observed `go test -v` events — never on the presence of a string in a source
// file (which a no-op could satisfy by pasting the func name into a comment).
//
// Predicate C1222-003 is the anti-vacuity control: it proves this file's own
// harness reports "no such test" as a failure, so the PASS-line assertions in
// 001/002/004 cannot pass vacuously.
package cycle1222

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
// never a `./...` sweep, never one of the slow suites (./internal/core,
// ./cmd/evolve).
const acssuitePkg = "./internal/acssuite"

// goModuleDir resolves <tree>/go from THIS predicate file's own location
// (<tree>/go/acs/cycle1222/predicates_test.go → ../..). Deriving it from the
// file rather than the process cwd means the predicate always exercises the
// tree that holds it — main tree, cycle worktree, or fleet lane alike.
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

// goTestRun executes `go test -count=1 -v [-run pattern] <pkg>` in the module
// dir and returns the combined output plus the exit code. The context is a HANG
// guard only (no assertion is derived from elapsed time). cmd.Dir — never
// process cwd — pins the tree under test.
func goTestRun(t *testing.T, moduleDir, runPattern string) (out string, code int) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go toolchain not on PATH: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	args := []string{"test", "-count=1", "-v"}
	if runPattern != "" {
		args = append(args, "-run", runPattern)
	}
	args = append(args, acssuitePkg)

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

// TestC1222_001_ReachabilityProbeGuardRunsAndPasses drives the real toolchain
// against the real package and requires `TestCurrentCycleScopeReachable` to
// actually execute and pass.
//
// RED today: the guard does not exist, so `go test -run` emits
// "testing: warning: no tests to run" and never emits the PASS event.
func TestC1222_001_ReachabilityProbeGuardRunsAndPasses(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, "^"+reachabilityTestName+"$")

	if strings.Contains(out, "no tests to run") {
		t.Errorf("C1222-001: `go test -run ^%s$ %s` matched NOTHING — the reachability guard "+
			"(task acs-current-cycle-scope-reachability-probe) is absent from the package.\noutput:\n%s",
			reachabilityTestName, acssuitePkg, excerpt(out))
	}
	if !strings.Contains(out, passLine(reachabilityTestName)) {
		t.Errorf("C1222-001: expected event %q in `go test -v` output; the guard must RUN and PASS "+
			"(exit=%d).\noutput:\n%s", passLine(reachabilityTestName), code, excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1222-001: `go test -run ^%s$ %s` exited %d, want 0.\noutput:\n%s",
			reachabilityTestName, acssuitePkg, code, excerpt(out))
	}
}

// TestC1222_002_CycleNumberDriftGuardRunsAndPasses requires the adversarial
// drift case to actually execute and pass.
//
// RED today: `TestGoLanePatterns_CycleNumberDrift` does not exist.
func TestC1222_002_CycleNumberDriftGuardRunsAndPasses(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, "^"+driftTestName+"$")

	if strings.Contains(out, "no tests to run") {
		t.Errorf("C1222-002: `go test -run ^%s$ %s` matched NOTHING — the drift adversarial case "+
			"(task acs-scope-cyclenum-drift-adversarial-case) is absent.\noutput:\n%s",
			driftTestName, acssuitePkg, excerpt(out))
	}
	if !strings.Contains(out, passLine(driftTestName)) {
		t.Errorf("C1222-002: expected event %q in `go test -v` output; the guard must RUN and PASS "+
			"(exit=%d).\noutput:\n%s", passLine(driftTestName), code, excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1222-002: `go test -run ^%s$ %s` exited %d, want 0.\noutput:\n%s",
			driftTestName, acssuitePkg, code, excerpt(out))
	}
}

// TestC1222_003_HarnessRejectsAbsentGuard is the NEGATIVE / anti-vacuity
// control for this predicate file: a test name that must never exist has to
// produce the "no tests to run" warning and zero PASS events.
//
// If this predicate ever goes RED, the PASS-line assertions in 001/002/004 are
// not evidence of anything — `go test -run` would be matching (or reporting)
// something other than what those predicates believe. It is deliberately
// independent of whether the two deliverables landed.
func TestC1222_003_HarnessRejectsAbsentGuard(t *testing.T) {
	moduleDir := goModuleDir(t)
	const sentinel = "TestC1222SentinelThatMustNeverExist"

	out, code := goTestRun(t, moduleDir, "^"+sentinel+"$")

	if !strings.Contains(out, "no tests to run") {
		t.Errorf("C1222-003: harness unsound — `go test -run ^%s$` on a nonexistent test did NOT warn "+
			"\"no tests to run\"; the PASS-line assertions in C1222-001/002/004 cannot be trusted.\noutput:\n%s",
			sentinel, excerpt(out))
	}
	if strings.Contains(out, "--- PASS: "+sentinel) {
		t.Errorf("C1222-003: harness unsound — a nonexistent test reported a PASS event.\noutput:\n%s", excerpt(out))
	}
	if code != 0 {
		t.Errorf("C1222-003: `go test -run ^%s$ %s` exited %d; the package itself must build and the "+
			"empty selection must be a clean exit.\noutput:\n%s", sentinel, acssuitePkg, code, excerpt(out))
	}
}

// TestC1222_004_GuardsRunInDefaultUntaggedSuite is the WIRING/REACHABILITY
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
func TestC1222_004_GuardsRunInDefaultUntaggedSuite(t *testing.T) {
	moduleDir := goModuleDir(t)
	out, code := goTestRun(t, moduleDir, "")

	for _, name := range []string{reachabilityTestName, driftTestName} {
		if !strings.Contains(out, passLine(name)) {
			t.Errorf("C1222-004: %q did not run+pass in the DEFAULT untagged suite "+
				"(`go test -count=1 -v %s`, no -tags and no -run). A guard CI never executes is dead "+
				"code — check it is not behind `//go:build acs` or an excluded file.\noutput:\n%s",
				name, acssuitePkg, excerpt(out))
		}
	}
	if code != 0 {
		t.Errorf("C1222-004: the full `%s` package suite exited %d, want 0 — the new guards must not "+
			"regress the package.\noutput:\n%s", acssuitePkg, code, excerpt(out))
	}
}

// excerpt bounds pasted subprocess output so a failure message stays readable.
func excerpt(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}
