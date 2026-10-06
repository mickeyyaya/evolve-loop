//go:build integration

package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

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
# Real-REPL discipline: a consumed line clears the input and a fresh prompt is
# drawn — without the re-printed marker the pane reads as "parked" to
# submit-verify forever (the v22.20.0 release-red harness/heuristic mismatch).
while IFS= read -r line; do
  case "$line" in
    ARTIFACT=*) artifact="${line#ARTIFACT=}" ;;
    PROCEED)    [ -n "$artifact" ] && printf 'INJECTED-OK' > "$artifact" ;;
  esac
  printf '%%s\n' "$marker"
done
`, marker)
	path := filepath.Join(dir, "fake-await-inject.sh")
	fakeclitest.Install(t, path, script)
	return path
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

	// External sender: queue the unblocking command as `evolve bridge send
	// --workspace=<ws> --agent=itest "PROCEED"` would. Re-append idempotently
	// until the artifact appears, since a send that lands before the driver's
	// inbox cursor reaches EOF is skipped as backlog.
	runEnded := make(chan struct{})
	senderStopped := make(chan struct{})
	go func() {
		defer close(senderStopped)
		for !fileNonEmpty(cfg.Artifact) {
			_ = inbox.Append(cfg.Workspace, cfg.Agent, inbox.Envelope{
				Kind: inbox.KindCommand, Body: "PROCEED", Source: "cli",
			}, time.Now)
			select {
			case <-runEnded:
				return
			case <-time.After(150 * time.Millisecond):
			}
		}
	}()

	code, err := runTmuxREPL(context.Background(), cfg, itDeps(120*time.Millisecond),
		itLaunch(sess, launchCmd, marker, 0, false))
	close(runEnded)
	<-senderStopped
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
