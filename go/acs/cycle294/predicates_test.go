//go:build acs

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

var (
	cancelOnce sync.Once
	cancelOut  string
)

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

func funcCoverage(t *testing.T, pkg, fn string) (float64, string) {
	t.Helper()
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-coverprofile="+prof, pkg)
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	for _, ln := range strings.Split(funcOut, "\n") {
		fields := strings.Fields(ln)
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

func TestC294_001_GuardRefusesRelativeWorktreeBase(t *testing.T) {
	const relBase = "c294-relbase-probe"
	defer os.RemoveAll(relBase)

	projectRoot := t.TempDir()
	_, err := swarm.NewGitWorkerProvisioner(nil, relBase).CreateIntegration(context.Background(), projectRoot, 294)
	if err == nil {
		t.Fatalf("RED: a relative worktree.base override %q must be refused, got nil error", relBase)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("RED: guard absent — provisioner error %q does not indicate the worktree base must be absolute", err.Error())
	}
}

func TestC294_002_SwarmrunnerSuiteLeavesNoRepoWorktrees(t *testing.T) {
	dir := goDir(t)
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
