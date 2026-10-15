//go:build integration

package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

var itTmuxCtl = execTmux{}

func requireTmux(t *testing.T) {
	t.Helper()
	testhost.Tool(t, "tmux")
}

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

func itDeps(perTick time.Duration) Deps {
	d := Deps{
		Tmux:  execTmux{},
		Sleep: func(time.Duration) { time.Sleep(perTick) },
		Now:   time.Now,
	}
	return d.withDefaults()
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

func itSession(tag string) string {
	return fmt.Sprintf("evolve-bridge-it-%s-%d", tag, os.Getpid())
}

func TestRealTmux_HappyPath(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	const marker = "READY-IT"
	cfg := itConfig(t, "ARTIFACT="+"PLACEHOLDER")
	mustWrite(t, cfg.PromptFile, "ARTIFACT="+cfg.Artifact)

	launchCmd := writeFakeREPL(t, cfg.Worktree, "happy", marker)
	sess := itSession("happy")
	deps := itDeps(120 * time.Millisecond)
	var stderr bytes.Buffer
	deps.Stderr = &stderr
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, err := runTmuxREPL(context.Background(), cfg, deps, itLaunch(sess, launchCmd, marker, 0, false))
	if err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "re-sending Enter") {
		t.Fatalf("clean-REPL fake triggered submit-verify resends; stderr=%s", stderr.String())
	}
	if got := readFile(t, cfg.Artifact); got != "PONG" {
		t.Fatalf("artifact = %q, want PONG", got)
	}
	if itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("ephemeral session %s should be killed after the run", sess)
	}
	if !fileNonEmpty(cfg.StdoutLog) {
		t.Fatalf("stdout-log scrollback should have been captured")
	}
}

func TestRealTmux_BootTimeout(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	launchCmd := writeFakeREPL(t, cfg.Worktree, "boot-timeout", "NEVER")
	sess := itSession("boot")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(10*time.Millisecond),
		itLaunch(sess, launchCmd, "NEVER", 0, false))
	if code != ExitREPLBootTimeout {
		t.Fatalf("exit = %d, want %d (ExitREPLBootTimeout)", code, ExitREPLBootTimeout)
	}
	if itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("session should be cleaned up after boot timeout")
	}
}

func TestRealTmux_ArtifactTimeout(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	const marker = "AT-READY"
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	cfg.ArtifactTimeoutS = 4
	launchCmd := writeFakeREPL(t, cfg.Worktree, "artifact-timeout", marker)
	sess := itSession("artto")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(150*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, false))
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout)", code, ExitArtifactTimeout)
	}
	if itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("ephemeral session should be cleaned up after artifact timeout")
	}
}

func TestRealTmux_MultilineSpecialCharPrompt(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	const marker = "ML-READY"
	const special = `a"b'c;d $x |f &g`
	cfg := itConfig(t, "x")
	mustWrite(t, cfg.PromptFile, "SPECIAL="+special+"\nARTIFACT="+cfg.Artifact)
	launchCmd := writeFakeREPL(t, cfg.Worktree, "happy", marker)
	sess := itSession("multiline")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(150*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, false))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if got := readFile(t, cfg.Artifact); got != special {
		t.Fatalf("artifact = %q, want the special string %q delivered intact", got, special)
	}
}

func TestRealTmux_NamedSessionResume(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	const marker = "NS-READY"
	sess := itSession("named")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	cfg1 := itConfig(t, "x")
	mustWrite(t, cfg1.PromptFile, "ARTIFACT="+cfg1.Artifact)
	launchCmd := writeFakeREPL(t, cfg1.Worktree, "happy", marker)
	code1, _ := runTmuxREPL(context.Background(), cfg1, itDeps(120*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, true))
	if code1 != ExitOK {
		t.Fatalf("run 1 exit = %d, want ExitOK", code1)
	}
	if !itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("named session must be PRESERVED after run 1")
	}
	if got := readFile(t, cfg1.Artifact); got != "PONG" {
		t.Fatalf("run 1 artifact = %q, want PONG", got)
	}

	cfg2 := itConfig(t, "x")
	mustWrite(t, cfg2.PromptFile, "ARTIFACT="+cfg2.Artifact)
	code2, _ := runTmuxREPL(context.Background(), cfg2, itDeps(120*time.Millisecond),
		itLaunch(sess, "false", marker, 0, true))
	if code2 != ExitOK {
		t.Fatalf("run 2 (resume) exit = %d, want ExitOK", code2)
	}
	if got := readFile(t, cfg2.Artifact); got != "PONG" {
		t.Fatalf("run 2 artifact = %q, want PONG (resume delivered a new prompt)", got)
	}
	if !itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("named session must still be preserved after resume")
	}
}

func TestRealTmux_ConcurrentSessionsIsolated(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	const marker = "CC-READY"
	const n = 3
	type run struct {
		cfg       *Config
		sess      string
		want      string
		launchCmd string
	}
	runs := make([]run, n)
	for i := 0; i < n; i++ {
		cfg := itConfig(t, "x")
		want := fmt.Sprintf("PAYLOAD-%d", i)
		mustWrite(t, cfg.PromptFile, "SPECIAL="+want+"\nARTIFACT="+cfg.Artifact)
		runs[i] = run{
			cfg:       cfg,
			sess:      itSession(fmt.Sprintf("cc%d", i)),
			want:      want,
			launchCmd: writeFakeREPL(t, cfg.Worktree, "happy", marker),
		}
	}
	defer func() {
		for _, r := range runs {
			_ = itTmuxCtl.KillSession(context.Background(), r.sess)
		}
	}()

	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = runTmuxREPL(context.Background(), runs[i].cfg,
				itDeps(150*time.Millisecond), itLaunch(runs[i].sess, runs[i].launchCmd, marker, 0, false))
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if codes[i] != ExitOK {
			t.Fatalf("run %d exit = %d, want ExitOK", i, codes[i])
		}
		if got := readFile(t, runs[i].cfg.Artifact); got != runs[i].want {
			t.Fatalf("run %d artifact = %q, want %q (prompt cross-contamination?)", i, got, runs[i].want)
		}
	}
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

func TestRealTmux_NewSessionInBindsPaneCwd(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	ctx := context.Background()
	workdir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", workdir, err)
	}
	session := fmt.Sprintf("it-cb2-birthcwd-%d", os.Getpid())
	if err := itTmuxCtl.NewSessionIn(ctx, session, 200, 50, workdir); err != nil {
		t.Fatalf("NewSessionIn: %v", err)
	}
	t.Cleanup(func() { _ = itTmuxCtl.KillSession(ctx, session) })

	out, err := exec.Command("tmux", TmuxSocketArgs("display-message", "-p", "-t", session, "#{pane_current_path}")...).Output()
	if err != nil {
		t.Fatalf("display-message: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != canonical {
		t.Errorf("pane_current_path=%q, want %q — new-session -c did not bind the cwd at birth", got, canonical)
	}
}
