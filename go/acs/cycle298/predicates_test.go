//go:build acs

package cycle298

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe   = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe    = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	coverTotalRe = regexp.MustCompile(`(?m)^total:\s+\S+\s+([0-9.]+)%`)
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
	gcHookOnce sync.Once
	gcHookOut  string
)

func runGCHookTests(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	gcHookOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v",
			"-run", "TestGC", "./cmd/evolve/")
		gcHookOut = stdout + "\n" + stderr
	})
	return gcHookOut
}

func TestC298_002_GCHookBehaviorMatrix(t *testing.T) {
	out := runGCHookTests(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a TestGC* hook test FAILs:\n%s", tail(out, 50))
	}
	want := []string{
		"TestGCShadow",
		"TestGCOff",
		"TestGCEnforce",
		"TestGCInvalidMode",
		"TestGCShadowMissingRunsDir",
		"TestGCShadowLiveRunExcluded",
	}
	for _, name := range want {
		if !topLevelPassed(out, name) {
			t.Errorf("RED: %s did not PASS — runGCHook does not yet satisfy this mode", name)
		}
	}
}

func TestC298_002b_CmdEvolveSuiteGreen(t *testing.T) {
	dir := goDir(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1", "./cmd/evolve/")
	out := stdout + "\n" + stderr
	if anyFailRe.MatchString(out) || code != 0 {
		t.Errorf("RED/REGRESSION: cmd/evolve suite is not green (exit=%d):\n%s", code, tail(out, 50))
	}
}

func TestC298_003_GCCoverageFloor(t *testing.T) {
	const floor = 95.0
	dir := goDir(t)
	profile := filepath.Join(t.TempDir(), "gc.cover")

	if _, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-count=1",
		"-coverprofile="+profile, "./internal/gc/..."); err != nil || code != 0 {
		t.Fatalf("RED: internal/gc test run failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 40))
	}

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover failed (exit=%d, err=%v):\n%s", code, err, tail(stderr, 20))
	}
	m := coverTotalRe.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("could not parse total coverage from:\n%s", tail(stdout, 20))
	}
	pct, perr := strconv.ParseFloat(m[1], 64)
	if perr != nil {
		t.Fatalf("unparsable coverage %q: %v", m[1], perr)
	}
	if pct < floor {
		t.Errorf("RED: internal/gc coverage %.1f%% < %.1f%% floor — add the Apply/nowLive/"+
			"protected/dirEntriesOlderThan safety-path tests", pct, floor)
	}
}
