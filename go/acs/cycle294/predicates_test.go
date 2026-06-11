//go:build acs

// Package cycle294 materializes the cycle-294 acceptance criteria for the two
// committed top_n tasks (scout-report.md — swarm worktree-base isolation +
// dispatch semaphore-cancel coverage):
//
//	T1  swarm-worktree-test-isolation       — the swarmrunner writer-failure test
//	    ran the real WorkerProvisioner with ProjectRoot:"." and no
//	    EVOLVE_WORKTREE_BASE pin, so worktreeBase(".") returned the RELATIVE
//	    ".evolve/worktrees" and `git -C . worktree add` leaked
//	    cycle-1-{integration,w0,w1} into the LIVE repo every run. Fix: (a) a guard
//	    in addWorktree refuses a non-absolute base before touching git; (b) the
//	    test runs against an isolated temp git repo with an absolute base pin.
//	T2  swarm-dispatch-semaphore-cancel      — Dispatch's `case <-rootCtx.Done()`
//	    arm (a worker still queued on the bounded semaphore when a sibling's fatal
//	    failure cancels the root context) was uncovered (Dispatch func = 96.0%).
//	    Fix: a new test drives 3 workers at Concurrency:1 with a failing+slow w0 so
//	    a queued worker observes context.Canceled.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test — they call the real provisioner, run the real swarmrunner
// suite and inspect `git worktree list`, run `go test -v` and assert on the real
// `--- PASS:` lines, and read the real `go tool cover -func` Dispatch number. A
// magic string in a .go file can neither refuse a relative base, remove a
// registered worktree, produce a named PASS line, nor move a coverage number, so
// none of these is gameable by source editing alone.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.guard  addWorktree refuses a relative base                 → C294_001 (direct call)
//	T1.noleak swarmrunner suite leaves 0 repo worktrees           → C294_002 (suite + git)
//	T1.suite  full swarm/swarmrunner suite stays green            → manual+checklist (auditor)
//	T2.test   TestDispatch_CancelWhileQueuedOnSemaphore PASSes    → C294_003 (PASS line)
//	T2.cover  Dispatch function coverage >= 97%                   → C294_004 (cover -func)
package cycle294

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` / `git -C <goDir>` make every
// invocation cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

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

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// --- shared one-shot subprocess runners (one `go test` per scope, reused) ---

var (
	cancelOnce sync.Once
	cancelOut  string
)

// runSwarmCancel runs ONLY the T2 cancel test, verbose, ONCE per predicate
// process. Scoped via -run so an unrelated swarm regression cannot false-RED this
// gate. The swarm package tests are worktree-isolated (provision_test uses
// gitInit; the dispatcher tests use no provisioner), so this never touches the
// live repo's worktree list.
func runSwarmCancel(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	cancelOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestDispatch_CancelWhileQueuedOnSemaphore", "./internal/swarm/")
		cancelOut = stdout + "\n" + stderr
	})
	return cancelOut
}

// funcCoverage runs the package suite with -coverprofile and returns the
// statement-coverage percentage `go tool cover -func` reports for the named
// function. The number is produced by REALLY running the package tests, so it can
// only move once the builder's new test exercises the real cancel arm —
// un-gameable by source edits.
func funcCoverage(t *testing.T, pkg, fn string) (float64, string) {
	t.Helper()
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-coverprofile="+prof, pkg)
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	for _, ln := range strings.Split(funcOut, "\n") {
		fields := strings.Fields(ln)
		// `go tool cover -func` row: "<file>:<line>:" "<FuncName>" "<pct>%"
		if len(fields) < 3 || fields[1] != fn {
			continue
		}
		pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
		if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
			return pct, ""
		}
	}
	return -1, "test stderr:\n" + tail(tErr, 20) + "\ncover stderr:\n" + tail(cErr, 20)
}

// ===================== T1 — swarm worktree-base isolation ====================

// --- C294_001 (T1.guard): addWorktree refuses a NON-ABSOLUTE worktree base ---
//
// Behavioral: calls the real gitWorkerProvisioner via CreateIntegration with a
// RELATIVE EVOLVE_WORKTREE_BASE. The guard must reject it BEFORE any git/MkdirAll
// runs, returning an error that identifies the base must be absolute — that early
// refusal is precisely what stops a relative base from polluting the live repo.
//
// RED baseline: no guard exists, so worktreeBase returns the relative string and
// addWorktree proceeds to `git worktree add <relpath>` (which errors against a
// non-repo temp root with a *git* message that does NOT mention "absolute"), so
// the discriminating assertion fails. The deferred RemoveAll sweeps the relative
// dir an un-guarded build creates under cwd, keeping the RED run side-effect-free.
func TestC294_001_GuardRefusesRelativeWorktreeBase(t *testing.T) {
	const relBase = "c294-relbase-probe" // relative → the bug class
	t.Setenv("EVOLVE_WORKTREE_BASE", relBase)
	defer os.RemoveAll(relBase) // sweep what an un-guarded (RED) impl MkdirAll's under cwd

	projectRoot := t.TempDir() // intentionally NOT a git repo
	_, err := swarm.NewGitWorkerProvisioner(nil).CreateIntegration(context.Background(), projectRoot, 294)
	if err == nil {
		t.Fatalf("RED: a relative EVOLVE_WORKTREE_BASE %q must be refused, got nil error", relBase)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("RED: guard absent — provisioner error %q does not indicate the worktree base must be absolute", err.Error())
	}
}

// --- C294_002 (T1.noleak): running the swarmrunner suite leaks 0 repo worktrees -
//
// The strongest anti-no-op signal and the scout verifiableBy
// (`git worktree list | grep swarmrunner | wc -l == 0`). Runs the real swarmrunner
// suite (the writer-failure test drives the real provisioner in writer+enforce
// mode) and then inspects the LIVE `git worktree list`. A registered worktree is a
// real side effect no source string can fake or erase.
//
// RED baseline: cycle-1-{integration,w0,w1} are already registered (scout
// confirmed 3 leaked) and the unfixed test re-leaks; GREEN requires BOTH the
// isolated-temp-repo test rewrite AND removal of the 3 stale worktrees.
func TestC294_002_SwarmrunnerSuiteLeavesNoRepoWorktrees(t *testing.T) {
	dir := goDir(t)
	// Run the suite that historically leaked. With the fix it provisions only inside
	// an isolated temp git repo (auto-removed); without it, it touches the live repo.
	_, _, _, _ = acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "./internal/phases/swarmrunner/")

	out, _, _, _ := acsassert.SubprocessOutput("git", "-C", dir, "worktree", "list")
	var leaked []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "swarmrunner") {
			leaked = append(leaked, strings.TrimSpace(ln))
		}
	}
	if len(leaked) != 0 {
		t.Errorf("RED: %d swarmrunner worktree(s) registered in the repo after the suite ran "+
			"— test isolation (absolute EVOLVE_WORKTREE_BASE in an isolated git repo) not in place:\n%s",
			len(leaked), strings.Join(leaked, "\n"))
	}
}

// ===================== T2 — dispatch semaphore-cancel coverage ===============

// --- C294_003 (T2.test): the semaphore-cancel test exists and PASSES ----------
//
// Behavioral: gates on a real `--- PASS: TestDispatch_CancelWhileQueuedOnSemaphore`
// line. That test (scout T2) drives 3 workers at Concurrency:1 with a failing+slow
// w0 so a worker still queued on the semaphore observes context.Canceled when the
// root context is cancelled — exercising the `case <-rootCtx.Done()` arm. A magic
// string cannot produce a named PASS line. RED: the test does not exist → no line.
func TestC294_003_DispatchSemaphoreCancelTestPasses(t *testing.T) {
	out := runSwarmCancel(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: the semaphore-cancel test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestDispatch_CancelWhileQueuedOnSemaphore") {
		t.Errorf("RED: TestDispatch_CancelWhileQueuedOnSemaphore did not PASS — the " +
			"`case <-rootCtx.Done()` semaphore-cancel arm of Dispatch is not yet exercised")
	}
}

// --- C294_004 (T2.cover): Dispatch function coverage reaches the >= 97% floor --
//
// Objective/un-gameable: the number comes from really running the swarm suite with
// -coverprofile and reading the Dispatch row of `go tool cover -func`. RED baseline
// (scout-294): Dispatch = 96.0% (the lone uncovered statement is the queued-worker
// `WorkerResult{..., Err: rootCtx.Err()}` inside `case <-rootCtx.Done()`).
func TestC294_004_DispatchFunctionCoverageFloor(t *testing.T) {
	pct, diag := funcCoverage(t, "./internal/swarm/", "Dispatch")
	if pct < 0 {
		t.Fatalf("RED: no `Dispatch` row from `go tool cover -func` for internal/swarm — profile not produced.\n%s", diag)
	}
	if pct < 97.0 {
		t.Errorf("RED: Dispatch coverage = %.1f%%, want >= 97.0%% (baseline 96.0%%; the "+
			"`case <-rootCtx.Done()` semaphore-cancel statement must be covered)", pct)
	}
}
