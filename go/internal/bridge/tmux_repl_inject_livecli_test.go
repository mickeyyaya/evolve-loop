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
	t.Parallel()
	requireTmux(t)
	const marker = "INJECT-READY"
	cfg := itConfig(t, "ARTIFACT=PLACEHOLDER")
	mustWrite(t, cfg.PromptFile, "ARTIFACT="+cfg.Artifact)
	cfg.ArtifactTimeoutS = 60

	launchCmd := writeAwaitInjectFake(t, cfg.Worktree)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sess := itSession("einject")
	defer itTmuxCtl.KillSession(context.Background(), sess)

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

	code, err := runTmuxREPL(ctx, cfg, itDeps(120*time.Millisecond),
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
