package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// tmux_repl_integration_test.go — HEAVY edge-case coverage of the bridge's
// core value: driving a real CLI REPL through a real tmux server.
//
// Unlike driver_claudetmux_test.go (which drives the runTmuxREPL state
// machine with a *fake* TmuxController), these tests run the FULL
// runTmuxREPL flow against a REAL tmux server (execTmux) with a scripted
// fake-CLI standing in for claude/codex/agy. That exercises the parts a
// fake can never prove: that real `capture-pane` actually contains the
// boot marker when the CLI is ready, that `load-buffer`+`paste-buffer`
// actually delivers the prompt (multi-line, special chars) to the REPL's
// stdin, that the artifact-wait observes a real on-disk write, and that
// session lifecycle (named preserve/resume, ephemeral kill, concurrent
// isolation) behaves on a real server.
//
// Speed: the Tmux and Sleep seams are independent, so we keep real tmux
// but inject a scaled-down Sleep. Boot/artifact-timeout cases use a tiny
// sleep (marker/artifact never appear, so rendering time is irrelevant);
// happy-path cases use a slightly larger sleep so real tmux has wall-time
// to render between launch and the first capture-pane poll.

// itTmuxCtl is the production tmux controller used directly for setup /
// teardown assertions (HasSession, KillSession). execTmux is stateless, so
// one shared value is safe; a named var also avoids the `itTmuxCtl.M()`
// composite-literal ambiguity inside if/for conditions.
var itTmuxCtl = execTmux{}

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; skipping real-tmux REPL integration coverage")
	}
}

// writeFakeREPL writes a scripted fake CLI to dir and returns its launch
// command. mode selects behavior; marker is the boot-ready string the fake
// prints once on startup.
//
// The marker is BAKED INTO the script body, never passed as an argv token —
// otherwise the shell would echo the launch command (which contains the
// marker) into the pane, and capture-pane would "detect" the marker from
// the echoed command line before the CLI ever started (a false boot-ready).
// Real markers (❯, ›) avoid this naturally because they aren't substrings
// of `claude --model …`; the fake must be equally careful.
//
// The fake reads pasted prompt lines from stdin and acts on the
// ARTIFACT=<path> / SPECIAL=<value> directives the test prompt carries.
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
# Read pasted prompt lines until the session is killed.
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
done
`, marker)
	path := filepath.Join(dir, "fake-repl-"+mode+".sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake repl: %v", err)
	}
	// launch command carries ONLY the mode — never the marker (see above).
	return fmt.Sprintf("%s %s", path, mode)
}

// itConfig builds a fully-populated Config rooted under a fresh temp dir,
// with the prompt file carrying the given lines (joined by "\n").
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
		Model:      "haiku",
		Agent:      "itest",
		Cycle:      0,
		Worktree:   root, // a real dir for the REPL to cd into
		Workspace:  ws,
		PromptFile: promptFile,
		Artifact:   filepath.Join(root, "artifact"),
		StdoutLog:  filepath.Join(ws, "stdout.log"),
		StderrLog:  filepath.Join(ws, "stderr.log"),
	}
}

// itDeps returns real-tmux Deps with a scaled Sleep. perTick is the wall
// time each poll-loop Sleep actually blocks (regardless of requested d).
func itDeps(perTick time.Duration) Deps {
	d := Deps{
		Tmux:  execTmux{},
		Sleep: func(time.Duration) { time.Sleep(perTick) },
		Now:   time.Now,
	}
	return d.withDefaults()
}

// itLaunch is a tmuxLaunch with the synthetic name "itest-tmux" (no
// manifest → auto-responder is a no-op, so the fake's output never trips
// an unexpected auto-response).
func itLaunch(session, launchCmd, marker string, scrollback int, named bool) tmuxLaunch {
	return tmuxLaunch{
		name:           "itest-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmd,
		promptMarker:   marker,
		bootScrollback: scrollback,
		bootIntervalS:  1,
		exitSeq:        []tmuxKey{{keys: "/exit", enter: true, pauseS: 0}},
	}
}

func itSession(tag string) string {
	return fmt.Sprintf("evolve-bridge-it-%s-%d", tag, os.Getpid())
}

// --- 1. Happy path end-to-end against real tmux ---------------------------

func TestRealTmux_HappyPath(t *testing.T) {
	requireTmux(t)
	const marker = "READY-IT"
	cfg := itConfig(t, "ARTIFACT="+"PLACEHOLDER")
	// Rewrite the prompt with the real artifact path now that cfg exists.
	mustWrite(t, cfg.PromptFile, "ARTIFACT="+cfg.Artifact)

	launchCmd := writeFakeREPL(t, cfg.Worktree, "happy", marker)
	sess := itSession("happy")
	deps := itDeps(120 * time.Millisecond)
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, err := runTmuxREPL(context.Background(), cfg, deps, itLaunch(sess, launchCmd, marker, 0, false))
	if err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
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

// --- 2. Boot timeout (real tmux, marker never appears) → EC 80 ------------

func TestRealTmux_BootTimeout(t *testing.T) {
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

// --- 3. Artifact timeout (REPL boots, artifact never written) → EC 81 -----

func TestRealTmux_ArtifactTimeout(t *testing.T) {
	requireTmux(t)
	const marker = "AT-READY"
	// The artifact-timeout fake never writes regardless of the ARTIFACT
	// directive, so the path is a don't-care; runTmuxREPL still polls the
	// real cfg.Artifact (set by itConfig) and never sees it appear.
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	launchCmd := writeFakeREPL(t, cfg.Worktree, "artifact-timeout", marker)
	sess := itSession("artto")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(10*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, false))
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout)", code, ExitArtifactTimeout)
	}
	if itTmuxCtl.HasSession(context.Background(), sess) {
		t.Fatalf("ephemeral session should be cleaned up after artifact timeout")
	}
}

// --- 4. Multi-line + special-char prompt delivery via paste-buffer --------

func TestRealTmux_MultilineSpecialCharPrompt(t *testing.T) {
	requireTmux(t)
	const marker = "ML-READY"
	const special = `a"b'c;d $x |f &g`
	cfg := itConfig(t, "x")
	// SPECIAL line then ARTIFACT line — the fake echoes SPECIAL into the
	// artifact, so an exact match proves multi-line + special chars were
	// delivered intact (paste-buffer, not send-keys, preserves them).
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

// --- 5. Named session: create → preserve → resume(reattach) → kill --------

func TestRealTmux_NamedSessionResume(t *testing.T) {
	requireTmux(t)
	const marker = "NS-READY"
	sess := itSession("named")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	// Run 1: create-named. Session must PERSIST after the run (no kill,
	// exit seq skipped).
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

	// Run 2: resume. Same session already exists → reattach, skip relaunch,
	// deliver a new prompt to the still-running fake REPL, get a new
	// artifact at a fresh path.
	cfg2 := itConfig(t, "x")
	mustWrite(t, cfg2.PromptFile, "ARTIFACT="+cfg2.Artifact)
	// "false" as launchCmd proves the resume path NEVER relaunches: if it
	// did, `false` would exit non-zero, no marker, boot timeout. Success here
	// means runTmuxREPL reattached to the run-1 REPL instead.
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

// --- 6. Concurrent sessions: each gets its OWN artifact content -----------
// Exercises real parallel tmux sessions on the shared server. Guards
// against prompt cross-contamination between concurrent launches.

func TestRealTmux_ConcurrentSessionsIsolated(t *testing.T) {
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
		// Each session must write ITS OWN payload to ITS OWN artifact.
		mustWrite(t, cfg.PromptFile, "SPECIAL="+want+"\nARTIFACT="+cfg.Artifact)
		// Write the fake CLI HERE (main goroutine) — writeFakeREPL calls
		// t.Fatalf, which is illegal from a non-test goroutine and would
		// silently swallow a write error if invoked inside the goroutine below.
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

	// Under the OLD global-buffer LoadBuffer/PasteBuffer, the concurrent
	// load-buffers raced and every paste pulled whichever buffer was loaded
	// last — so the per-run artifact assertion below would see the WRONG
	// payload. With session-scoped buffers (tmux.go), each run gets its own.

	for i := 0; i < n; i++ {
		if codes[i] != ExitOK {
			t.Fatalf("run %d exit = %d, want ExitOK", i, codes[i])
		}
		if got := readFile(t, runs[i].cfg.Artifact); got != runs[i].want {
			t.Fatalf("run %d artifact = %q, want %q (prompt cross-contamination?)", i, got, runs[i].want)
		}
	}
}

// --- small file helpers ----------------------------------------------------

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
