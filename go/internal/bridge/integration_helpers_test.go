//go:build integration

package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

func requireNativeSandbox(t *testing.T) sandbox.ProbeResult {
	t.Helper()
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		testhost.Unavailable(t, sandboxDiagnosis(probe))
	}
	return probe
}

const capabilityProbeSBPL = "(version 1)(allow default)"

func sandboxDiagnosis(probe sandbox.ProbeResult) string {
	if probe.OS != "darwin" || !probe.Available {
		return fmt.Sprintf("the agent sandbox is unavailable on %s: %s", probe.OS, probe.Reason)
	}
	out, err := exec.Command(probe.BinaryPath, "-p", capabilityProbeSBPL, "/usr/bin/true").CombinedOutput()
	return fmt.Sprintf("the agent sandbox cannot apply on this runner: binary=%s probe=%q apply error=%v output=%q SBPL=%q",
		probe.BinaryPath, probe.Reason, err, strings.TrimSpace(string(out)), capabilityProbeSBPL)
}

func writeFakeREPL(t *testing.T, dir, mode, marker string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -u
mode="$1"
marker=%q
case "$mode" in
  boot-timeout)
    # Never print the marker — the boot poll loop must exhaust → EC 80.
    exec sleep 300 ;;
  *)
    printf '%%s\n' "$marker" ;;
esac
# Read pasted prompt lines until the session is killed. After EVERY consumed
# line, re-print the input-line marker — real-REPL discipline: a submitted
# line clears the input and a fresh prompt is drawn. Without this, the pane's
# last marker keeps the echoed prompt as its tail forever and submit-verify
# correctly reads "parked" (the v22.20.0 release red was exactly this
# harness/heuristic mismatch).
special=""
while IFS= read -r line; do
  case "$line" in
    SPECIAL=*) special="${line#SPECIAL=}" ;;
    ARTIFACT=*)
      if [ "$mode" != "artifact-timeout" ]; then
        if [ -n "${special:-}" ]; then printf '%%s' "$special" > "${line#ARTIFACT=}"
        else printf 'PONG' > "${line#ARTIFACT=}"; fi
      fi ;;
  esac
  printf '%%s\n' "$marker"
done
`, marker)
	path := filepath.Join(dir, "fake-repl-"+mode+".sh")
	fakeclitest.Install(t, path, script)
	return path + " " + mode
}

func itConfig(t *testing.T, promptLines ...string) *Config {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	prompt := strings.Join(promptLines, "\n")
	promptFile := filepath.Join(root, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	return &Config{
		ProjectRoot: root,
		Model:       "haiku",
		Agent:       "itest",
		Cycle:       0,
		Worktree:    root,
		Workspace:   ws,
		PromptFile:  promptFile,
		Artifact:    filepath.Join(root, "artifact"),
		StdoutLog:   filepath.Join(ws, "stdout.log"),
		StderrLog:   filepath.Join(ws, "stderr.log"),
	}
}

func itLaunch(session, launchCmd, marker string, scrollback int, named bool) tmuxLaunch {
	return tmuxLaunch{
		name:            "itest-tmux",
		session:         session,
		named:           named,
		launchCmd:       launchCmd,
		promptMarker:    marker,
		inputLineMarker: marker,
		bootScrollback:  scrollback,
		bootIntervalS:   1,
		exitSeq:         []tmuxKey{{keys: "/exit", enter: true, pauseS: 0}},
	}
}

const itLaunchDeadline = 5 * time.Minute

func itBoundedContext(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func itSession(tag string) string {
	return fmt.Sprintf("evolve-bridge-it-%s-%d", tag, os.Getpid())
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
