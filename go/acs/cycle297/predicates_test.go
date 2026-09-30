//go:build acs

package cycle297

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
	freezeOnce sync.Once
	freezeOut  string
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

func runFreezeClaude(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	freezeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "VersionFreeze_Claude|DefaultSelfUpdateEvidence_Claude",
			"./internal/looppreflight/")
		freezeOut = stdout + "\n" + stderr
	})
	return freezeOut
}

func TestC297_001_WorktreeBaseRefusesRelativeDefaultRoot(t *testing.T) {
	out := runSwarmWorktreeBase(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestWorktreeBase* test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestWorktreeBase_RelativeProjectRootRefused") {
		t.Errorf("RED: TestWorktreeBase_RelativeProjectRootRefused did not PASS — worktreeBase " +
			"does not yet refuse a relative projectRoot on the default (no-env) path")
	}
}

func TestC297_001b_SwarmSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/swarm/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/swarm suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

func TestC297_002_FreezeRecognizesClaude(t *testing.T) {
	out := runFreezeClaude(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a claude version-freeze test FAILs:\n%s", tail(out, 40))
	}
	if !topLevelPassed(out, "TestDefaultSelfUpdateEvidence_ClaudePresent") {
		t.Errorf("RED: TestDefaultSelfUpdateEvidence_ClaudePresent did not PASS — " +
			"defaultSelfUpdateEvidence does not yet recognize claude's ~/.claude/settings.json")
	}
	if !topLevelPassed(out, "TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts") {
		t.Errorf("RED: TestRun_VersionFreeze_ClaudeUnpinnedRealEvidence_Halts did not PASS — " +
			"an unpinned self-updating claude-tmux does not yet HALT the batch")
	}
}

func TestC297_002b_LoopPreflightSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "-v", "./internal/looppreflight/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: internal/looppreflight suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}
