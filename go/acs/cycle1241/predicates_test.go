//go:build acs

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

const reachabilityTestName = "TestCurrentCycleScopeReachable"

const driftTestName = "TestGoLanePatterns_CycleNumberDrift"

const acssuitePkg = "./internal/acssuite"

const priorPredicatePkg = "./acs/cycle1222"

const priorPredicateRel = "go/acs/cycle1222/predicates_test.go"

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

func passLine(name string) string { return "--- PASS: " + name }

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

func TestC1241_005_PriorCycle1222PredicatesGoGreenUnmodified(t *testing.T) {
	moduleDir := goModuleDir(t)

	if _, err := os.Stat(filepath.Join(moduleDir, "acs", "cycle1222", "predicates_test.go")); err != nil {
		t.Fatalf("C1241-005: the cycle-1222 predicate spec is missing from this tree (%v) — it is the "+
			"acceptance spec for this todo and must NOT be deleted", err)
	}

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

func excerpt(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (truncated)"
}
