//go:build integration && !notmux

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

	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

var itTmuxCtl = execTmux{}

func requireTmux(t *testing.T) {
	t.Helper()
	testhost.Tool(t, "tmux")
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

	code, err := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg, deps, itLaunch(sess, launchCmd, marker, 0, false))
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

	code, _ := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg, itDeps(10*time.Millisecond),
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

	code, _ := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg, itDeps(150*time.Millisecond),
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

	code, _ := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg, itDeps(150*time.Millisecond),
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
	code1, _ := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg1, itDeps(120*time.Millisecond),
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
	code2, _ := runTmuxREPL(itBoundedContext(t, itLaunchDeadline), cfg2, itDeps(120*time.Millisecond),
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
			codes[i], _ = runTmuxREPL(itBoundedContext(t, itLaunchDeadline), runs[i].cfg,
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

	deadline := time.Now().Add(5 * time.Second)
	got := ""
	for {
		out, err := exec.Command("tmux", TmuxSocketArgs("display-message", "-p", "-t", session, "#{pane_current_path}")...).Output()
		if err != nil {
			t.Fatalf("display-message: %v", err)
		}
		got = strings.TrimSpace(string(out))
		if got == canonical || time.Now().After(deadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if got != canonical {
		t.Errorf("pane_current_path=%q, want %q — new-session -c did not bind the cwd at birth", got, canonical)
	}
}
