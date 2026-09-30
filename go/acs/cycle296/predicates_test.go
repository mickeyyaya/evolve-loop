//go:build acs

package cycle296

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

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

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
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
	swarmOnce  sync.Once
	swarmOut   string
	resumeOnce sync.Once
	resumeOut  string
)

func runSwarmWorktreeBase(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	swarmOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestWorktreeBase", "./internal/swarm/")
		swarmOut = stdout + "\n" + stderr
	})
	return swarmOut
}

func runResumeGuard(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	resumeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestRunCycleFromPhase", "./internal/core/")
		resumeOut = stdout + "\n" + stderr
	})
	return resumeOut
}

func TestC296_001_WorktreeBaseRefusesRelativeBase(t *testing.T) {
	out := runSwarmWorktreeBase(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestWorktreeBase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestWorktreeBase_RelativeOverrideReturnsError") {
		t.Errorf("RED: TestWorktreeBase_RelativeOverrideReturnsError did not PASS — worktreeBase " +
			"does not yet refuse a relative EVOLVE_WORKTREE_BASE itself (guard still only in addWorktree)")
	}
	prov := filepath.Join(goDir(t), "internal", "swarm", "provision.go")
	if n, err := acsassert.CountInGoFunc(prov, "addWorktree", "filepath.IsAbs"); err != nil || n != 0 {
		t.Errorf("addWorktree must contain NO filepath.IsAbs (guard lives in worktreeBase, "+
			"not duplicated): found %d, err=%v", n, err)
	}
	if n, err := acsassert.CountInGoFunc(prov, "worktreeBase", "filepath.IsAbs"); err != nil || n < 1 {
		t.Errorf("worktreeBase must carry the filepath.IsAbs guard itself: found %d, err=%v", n, err)
	}
}

func TestC296_002_SwarmSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/swarm/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/swarm suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

func TestC296_003_ResumeAcceptsInsertedPhase(t *testing.T) {
	out := runResumeGuard(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestRunCycleFromPhase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted") {
		t.Errorf("RED: TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted did not PASS — " +
			"the resume guard still rejects an advisor-inserted phase registered in o.runners")
	}
	if !topLevelPassed(out, "TestRunCycleFromPhase_PhaseStartRejected") {
		t.Errorf("RED/REGRESSION: TestRunCycleFromPhase_PhaseStartRejected did not PASS — " +
			"PhaseStart must remain a rejected resume target after the guard change")
	}
}
