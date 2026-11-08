//go:build acs

package cycle2230

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	flakyTestName    = "TestRealTmux_NewSessionInBindsPaneCwd"
	bridgePackage    = "./internal/bridge"
	currentFailText  = `pane_current_path=""`
	currentFailTail  = "new-session -c did not bind the cwd at birth"
	hundredRuns      = 100
	singleRunBound   = 4 * time.Minute
	hundredRunsBound = 9 * time.Minute
)

const shimScript = `#!/usr/bin/env bash
set -u
is_cwd_read=0
for a in "$@"; do
  if [ "$a" = "#{pane_current_path}" ]; then is_cwd_read=1; fi
done
if [ "$is_cwd_read" = "1" ]; then
  n=0
  if [ -f "$ACS_SHIM_COUNT" ]; then n=$(cat "$ACS_SHIM_COUNT"); fi
  n=$((n + 1))
  printf '%s' "$n" > "$ACS_SHIM_COUNT"
  if [ "$ACS_SHIM_MODE" = "always-empty" ]; then
    printf '\n'
    exit 0
  fi
  if [ $((n % 3)) -ne 0 ]; then
    printf '\n'
    exit 0
  fi
fi
exec "$ACS_REAL_TMUX" "$@"
`

type shimRun struct {
	output   string
	exitCode int
	reads    int
}

func goModuleDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("locate go module: %v", err)
	}
	return dir
}

func runIntegrationTest(t *testing.T, mode string, count int, race bool, bound time.Duration) shimRun {
	t.Helper()
	realTmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skipf("tmux is not installed: %v", err)
	}
	scratch := t.TempDir()
	shimDir := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatalf("mkdir shim dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shimDir, "tmux"), []byte(shimScript), 0o755); err != nil {
		t.Fatalf("write tmux shim: %v", err)
	}
	countFile := filepath.Join(scratch, "reads.count")

	args := []string{"test", "-tags", "integration", "-count=" + strconv.Itoa(count), "-run", "^" + flakyTestName + "$", "-timeout", bound.String()}
	if race {
		args = append(args, "-race")
	}
	args = append(args, bridgePackage)

	ctx, cancel := context.WithTimeout(context.Background(), bound+time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = goModuleDir(t)
	cmd.WaitDelay = 10 * time.Second
	cmd.Env = append(os.Environ(),
		"PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"ACS_REAL_TMUX="+realTmux,
		"ACS_SHIM_COUNT="+countFile,
		"ACS_SHIM_MODE="+mode,
	)
	out, runErr := cmd.CombinedOutput()
	code := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go test did not run: %v\n%s", runErr, acsassert.OutputTail(string(out), 20))
		}
	}
	reads := 0
	if raw, err := os.ReadFile(countFile); err == nil {
		reads, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
	}
	return shimRun{output: string(out), exitCode: code, reads: reads}
}

func TestC2230_001_PollsUntilPaneCwdAppears(t *testing.T) {
	run := runIntegrationTest(t, "empty-then-real", 1, false, singleRunBound)
	if run.exitCode != 0 {
		t.Errorf("RED: the test read pane_current_path while the shim reported it empty and failed (exit %d); it must poll until the value appears\n%s",
			run.exitCode, acsassert.OutputTail(run.output, 15))
	}
	if run.reads < 3 {
		t.Errorf("RED: only %d pane_current_path read(s) happened; polling past two empty reads needs at least 3", run.reads)
	}
}

func TestC2230_002_StillFailsWithCurrentMessageWhenValueStaysEmpty(t *testing.T) {
	run := runIntegrationTest(t, "always-empty", 1, false, singleRunBound)
	if run.exitCode == 0 {
		t.Fatalf("test passed although pane_current_path stayed empty; the assertion must still fail\n%s", acsassert.OutputTail(run.output, 15))
	}
	for _, want := range []string{currentFailText, currentFailTail} {
		if !strings.Contains(run.output, want) {
			t.Errorf("failure output lost the current message part %q\n%s", want, acsassert.OutputTail(run.output, 15))
		}
	}
	if run.reads < 2 {
		t.Errorf("RED: only %d read(s) of pane_current_path before the failure; a deadline poll reads repeatedly", run.reads)
	}
}

func TestC2230_003_PassesRaceCountHundredWithTransientEmptyReads(t *testing.T) {
	run := runIntegrationTest(t, "empty-then-real", hundredRuns, true, hundredRunsBound)
	if run.exitCode != 0 {
		t.Errorf("RED: go test -race -count=%d -tags integration failed (exit %d)\n%s",
			hundredRuns, run.exitCode, acsassert.OutputTail(run.output, 15))
	}
	if run.exitCode == 0 && !strings.Contains(run.output, "ok  \t") {
		t.Errorf("no ok line in go test output\n%s", acsassert.OutputTail(run.output, 10))
	}
}

func TestC2230_004_OrphanGuardCoversThisPredicatePackage(t *testing.T) {
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestEveryTmuxTestPackageUsesTheOrphanGuard$", "./internal/tmuxtest")
	cmd.Dir = goModuleDir(t)
	raw, runErr := cmd.CombinedOutput()
	out := string(raw)
	if runErr != nil {
		if _, isExit := runErr.(*exec.ExitError); !isExit {
			t.Fatalf("go test did not run: %v", runErr)
		}
		t.Errorf("RED: the orphan-guard scan fails; a package that starts tmux has no TestMain calling tmuxtest.Main\n%s", acsassert.OutputTail(out, 12))
	}
	if strings.Contains(out, "acs/cycle2230") {
		t.Errorf("RED: the scan still names acs/cycle2230 as unguarded\n%s", acsassert.OutputTail(out, 12))
	}
}
