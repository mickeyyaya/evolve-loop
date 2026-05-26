package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

// tmux_repl_inject_livecli_test.go — end-to-end proof of LIVE COMMAND
// INJECTION on a REAL tmux server. Unlike the deterministic fakeTmux tests
// (which record SendKeys but cannot exercise real paste-buffer I/O), this
// drives the full runTmuxREPL flow against execTmux: an external sender
// queues a command into the agent inbox mid-run, the driver drains it from
// the artifact-wait poll loop, and injects it via real `tmux load-buffer` +
// `paste-buffer` into the fake REPL's stdin.
//
// The fake is the linchpin: it boots, consumes the pasted prompt, and ONLY
// writes the artifact once it receives the injected "PROCEED" line. So a
// green test proves the injection actually reached the running agent; a
// broken inject path would mean the fake never gets PROCEED → no artifact →
// EC81 artifact-timeout → loud failure.

// writeAwaitInjectFake writes a fake CLI that blocks until a live-injected
// "PROCEED" command arrives, then writes a sentinel value (INJECTED-OK) the
// test can distinguish from any prompt-driven write. The marker is baked into
// the body (never argv) so tmux's echo of the launch command cannot leak it
// into the pane and trip a false boot-ready.
func writeAwaitInjectFake(t *testing.T, dir string) string {
	t.Helper()
	const marker = "INJECT-READY"
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -u
marker=%q
printf '%%s\n' "$marker"
artifact=""
while IFS= read -r line; do
  case "$line" in
    ARTIFACT=*) artifact="${line#ARTIFACT=}" ;;
    PROCEED)    [ -n "$artifact" ] && printf 'INJECTED-OK' > "$artifact" ;;
  esac
done
`, marker)
	path := filepath.Join(dir, "fake-await-inject.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write await-inject fake: %v", err)
	}
	return path // no args; behavior is baked in
}

func TestRealTmux_E2E_LiveInjection_UnblocksAgent(t *testing.T) {
	requireTmux(t)
	const marker = "INJECT-READY"
	cfg := itConfig(t, "ARTIFACT=PLACEHOLDER")
	// Rewrite the prompt with the real artifact path now that cfg exists.
	mustWrite(t, cfg.PromptFile, "ARTIFACT="+cfg.Artifact)
	cfg.ArtifactTimeoutS = 60 // bound failure: ~(60/2)*perTick real time

	launchCmd := writeAwaitInjectFake(t, cfg.Worktree)
	sess := itSession("einject")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	// External sender: queue the unblocking command exactly as
	// `evolve bridge send --workspace=<ws> --agent=itest "PROCEED"` would
	// (inbox.Append is the function that subcommand calls — see T2). Re-append
	// idempotently until the artifact appears: the driver seeks the cursor to
	// EOF on entry, so a send that lands before that point is skipped as
	// backlog; retrying guarantees one lands inside the wait loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if fileNonEmpty(cfg.Artifact) {
				return
			}
			_ = inbox.Append(cfg.Workspace, cfg.Agent, inbox.Envelope{
				Kind: inbox.KindCommand, Body: "PROCEED", Source: "cli",
			}, time.Now)
			time.Sleep(150 * time.Millisecond)
		}
	}()

	code, err := runTmuxREPL(context.Background(), cfg, itDeps(120*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, false))
	<-done
	if err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (injected PROCEED should unblock the agent)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "INJECTED-OK" {
		t.Fatalf("artifact = %q, want INJECTED-OK (only the injected command writes this — injection did not reach the REPL)", got)
	}
}
