//go:build acs

package cycle276

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	bridgeOnce   sync.Once
	bridgeOut    string
	recoveryOnce sync.Once
	recoveryOut  string
	runnerOnce   sync.Once
	runnerOut    string
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runBridge(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	bridgeOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/bridge/")
		bridgeOut = stdout + "\n" + stderr
	})
	return bridgeOut
}

func runRecovery(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	recoveryOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/recovery/")
		recoveryOut = stdout + "\n" + stderr
	})
	return recoveryOut
}

func runRunner(t *testing.T) string {
	t.Helper()
	dir := goDir(t)
	runnerOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", dir, "-count=1", "-v", "./internal/phases/runner/")
		runnerOut = stdout + "\n" + stderr
	})
	return runnerOut
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

func subPasses(out, parent string) map[string]bool {
	seen := map[string]bool{}
	prefix := parent + "/"
	for _, n := range passNames(out) {
		if strings.HasPrefix(n, prefix) {
			seen[strings.TrimPrefix(n, prefix)] = true
		}
	}
	return seen
}

func fixtureCases(out, prefix string) []string {
	names := passNames(out)
	parents := map[string]bool{}
	for _, n := range names {
		if i := strings.Index(n, "/"); i >= 0 {
			parents[n[:i]] = true
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if !strings.HasPrefix(n, prefix) {
			continue
		}
		if strings.Contains(n, "/") {
			seen[n] = true
			continue
		}
		if !parents[n] {
			seen[n] = true
		}
	}
	out2 := make([]string, 0, len(seen))
	for n := range seen {
		out2 = append(out2, n)
	}
	return out2
}

func TestC276_001_TmuxFixtureCorpusDrivesStateMachine(t *testing.T) {
	out := runBridge(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line:\n%s", tail(out, 60))
	}
	cases := fixtureCases(out, "TestTmuxFixture")
	if len(cases) < 3 {
		t.Errorf("RED: TestTmuxFixture* has %d passing case(s) %v, want >= 3 "+
			"(the FakeTmuxController-driven boot-success / boot-timeout / artifact-delivery corpus — scout T1 verifiableBy)",
			len(cases), cases)
	}
}

func TestC276_002_TmuxFixtureBootTimeoutNegative(t *testing.T) {
	out := runBridge(t)
	timeoutRe := regexp.MustCompile(`(?i)timeout`)
	for _, c := range fixtureCases(out, "TestTmuxFixture") {
		if timeoutRe.MatchString(c) {
			return
		}
	}
	t.Errorf("RED: no passing TestTmuxFixture case matches /timeout/ — the boot-timeout " +
		"negative (marker never appears → ExitREPLBootTimeout) is uncovered (T1.d)")
}

func TestC276_003_TmuxGoControllerCovered(t *testing.T) {
	dir := goDir(t)
	prof := filepath.Join(t.TempDir(), "cover.out")
	_, tErr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-short", "-count=1", "-coverprofile="+prof, "./internal/bridge/")
	funcOut, cErr, _, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+prof)
	covered, total := 0, 0
	for _, ln := range strings.Split(funcOut, "\n") {
		if !strings.Contains(ln, "/tmux.go:") {
			continue
		}
		total++
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		if fields[len(fields)-1] != "0.0%" {
			covered++
		}
	}
	if total == 0 {
		t.Fatalf("RED: no `/tmux.go:` rows in `go tool cover -func` output — profile not produced.\ntest stderr:\n%s\ncover stderr:\n%s",
			tail(tErr, 30), tail(cErr, 30))
	}
	if covered < 3 {
		t.Errorf("RED: tmux.go has %d/%d funcs with >0%% coverage, want >= 3 "+
			"(baseline 1 = stripANSI only; the FakeTmuxController / execTmux controller methods must be exercised by the fixture corpus — T1.c)",
			covered, total)
	}
}

func TestC276_004_CodexUpdateMenuDismissed(t *testing.T) {
	out := runBridge(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: bridge suite has a FAIL line (T2.d full-suite floor):\n%s", tail(out, 60))
	}
	if !topLevelPassed(out, "TestCodexUpdateMenuDismiss") {
		t.Errorf("RED: TestCodexUpdateMenuDismiss did not run+PASS — the codex boot-sync gate " +
			"(dismiss update-menu before injecting the prompt) is not implemented/tested (T2.a)")
	}
	if subs := subPasses(out, "TestCodexUpdateMenuDismiss"); len(subs) < 2 {
		t.Errorf("RED: TestCodexUpdateMenuDismiss has %d passing sub-case(s), want >= 2 "+
			"(menu-present → dismissed before inject, AND menu-absent → no spurious Skip keypress)", len(subs))
	}
}

func TestC276_005_FatalPaneShellSpillClassified(t *testing.T) {
	out := runBridge(t) + "\n" + runRecovery(t)
	if !topLevelPassed(out, "TestFatalPaneShellSpill") {
		t.Errorf("RED: TestFatalPaneShellSpill did not run+PASS — shell-spill panes " +
			"(quote>/bquote> continuation, command-not-found) are not classified fatal (T2.b)")
	}
	if subs := subPasses(out, "TestFatalPaneShellSpill"); len(subs) < 2 {
		t.Errorf("RED: TestFatalPaneShellSpill has %d passing sub-case(s), want >= 2 "+
			"(distinct spill signatures — e.g. a zsh quote>/bquote> continuation AND a command-not-found line)", len(subs))
	}
}

func TestC276_006_FatalPaneNoFalsePositive(t *testing.T) {
	out := runBridge(t) + "\n" + runRecovery(t)
	if !topLevelPassed(out, "TestFatalPaneNoFalsePositive") {
		t.Errorf("RED: TestFatalPaneNoFalsePositive did not run+PASS — the new shell-spill " +
			"signatures must NOT classify a healthy pane that merely mentions 'quote'/'command' as fatal (T2.c)")
	}
}

func TestC276_008_RunnerMissingProfileFastFail(t *testing.T) {
	out := runRunner(t)
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: runner suite has a FAIL line (T3.c full-suite floor — "+
			"the fast-fail must not break legitimately profile-less phases):\n%s", tail(out, 60))
	}
	if !topLevelPassed(out, "TestRunnerMissingProfileFastFail") {
		t.Errorf("RED: TestRunnerMissingProfileFastFail did not run+PASS — the runner still " +
			"tolerates a missing profile and defers the failure to the bridge's terse exit 10 (T3.a)")
	}
}

func TestC276_009_RunnerMissingProfileDiagnostic(t *testing.T) {
	out := runRunner(t)
	if !topLevelPassed(out, "TestRunnerMissingProfileDiagnostic") {
		t.Errorf("RED: TestRunnerMissingProfileDiagnostic did not run+PASS — the fast-fail " +
			"diagnostic must name the missing profile path (not a terse exit code) (T3.b)")
	}
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
