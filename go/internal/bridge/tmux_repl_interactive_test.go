//go:build integration

package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func writeInteractiveFake(t *testing.T, dir, marker, promptText, expect, timing, mode string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -uo pipefail
marker=%q
prompt_text=%q
expect=%q
timing=%q
mode=%q

# emit_and_wait prints the prompt then blocks until a stdin line equals $expect.
# For the bare-Enter responses (single-select, agy trust) $expect is "" — the
# auto-responder's Enter arrives as an empty line, which matches.
emit_and_wait() {
  printf '%%s\n' "$prompt_text"
  while IFS= read -r line; do
    [ "$line" = "$expect" ] && return 0
  done
}

[ "$timing" = boot ] && emit_and_wait
printf '%%s\n' "$marker"

artifact=""
while IFS= read -r line; do
  case "$line" in ARTIFACT=*) artifact="${line#ARTIFACT=}"; break ;; esac
done
# Drain the rest of the paste before printing anything: a canonical-mode read is slower than the
# paste, and a print in between interleaves with the echo and drops input (a real CLI reads raw).
while IFS= read -r -t 1 _; do :; done

if [ "$timing" = mid ]; then
  if [ "$mode" = persist ]; then
    printf '%%s\n' "$prompt_text"        # escalate / loop-guard: never clears
    while IFS= read -r _; do :; done     # runTmuxREPL returns on EC85 / EC86
  elif [ "$mode" = multiselect ]; then
    printf '%%s\n' "$prompt_text"        # contains '❯ N. [ ]' → multiselect rule
    # The submit needs the FULL Enter,Right,Enter sequence: two Enters delimit
    # two stdin lines (the Right arrow rides inside the second). A single-Enter
    # rule would deliver only one line → the second read blocks forever → no
    # artifact → EC81. So unblocking here proves the multi-keystroke delivery.
    IFS= read -r _                       # 1st Enter (toggle highlighted checkbox)
    IFS= read -r _                       # Right + 2nd Enter (navigate Submit + submit)
  else
    emit_and_wait
  fi
fi
# Reached here only after the auto-responder unblocked the prompt → continue.
[ -n "$artifact" ] && printf 'DONE' > "$artifact"
while IFS= read -r _; do :; done
`, marker, promptText, expect, timing, mode)
	path := filepath.Join(dir, "fake-interactive.sh")
	fakeclitest.Install(t, path, script)
	return path
}

func interactiveLaunch(cli, session, launchCmd, marker string, tickDuringBoot bool) tmuxLaunch {
	return tmuxLaunch{
		name:           cli,
		session:        session,
		launchCmd:      launchCmd,
		promptMarker:   marker,
		bootScrollback: 0,
		bootIntervalS:  1,
		tickDuringBoot: tickDuringBoot,
		exitSeq:        []tmuxKey{{keys: "/exit", enter: true}},
	}
}

func interactiveConfig(t *testing.T) *Config {
	t.Helper()
	cfg := itConfig(t, "x")
	mustWrite(t, cfg.PromptFile, "ARTIFACT=$ARTIFACT_PATH")
	return cfg
}

const itMarker = "ITREADY"

func TestRealTmux_Interactive_ClaudeSingleSelect_AutoEnter(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Pick one — Enter to select · up/down to navigate · Esc to cancel", "", "mid", "unblock")
	sess := itSession("isingle")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(250*time.Millisecond),
		interactiveLaunch("claude-tmux", sess, launch+" x", itMarker, false))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (auto-Enter should select the first option and continue)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "DONE" {
		t.Fatalf("artifact = %q, want DONE (workflow did not continue past the menu)", got)
	}
}

func TestRealTmux_Interactive_ClaudeMultiSelect_AutoSubmit(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Which toppings?\n❯ 1. [ ] Cheese\n  2. [ ] Mushroom\nEnter to select · up/down to navigate", "", "mid", "multiselect")
	sess := itSession("imulti")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(250*time.Millisecond),
		interactiveLaunch("claude-tmux", sess, launch+" x", itMarker, false))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (Enter,Right,Enter should toggle→navigate→submit and continue)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "DONE" {
		t.Fatalf("artifact = %q, want DONE (multi-select sequence did not submit → workflow stalled)", got)
	}
}

func TestRealTmux_Interactive_CodexTrust_AutoRespondAtBoot(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Do you trust the contents of this directory?", "1", "boot", "unblock")
	sess := itSession("itrust")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(250*time.Millisecond),
		interactiveLaunch("codex-tmux", sess, launch+" x", itMarker, true))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (trust dialog should be auto-dismissed at boot)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "DONE" {
		t.Fatalf("artifact = %q, want DONE (boot trust not dismissed → workflow stalled)", got)
	}
}

func TestRealTmux_Interactive_AgyPermission_AutoYes(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Allow write to /tmp/out.txt?", "y", "mid", "unblock")
	sess := itSession("iperm")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(250*time.Millisecond),
		interactiveLaunch("agy-tmux", sess, launch+" x", itMarker, false))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (permission prompt should be auto-approved)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "DONE" {
		t.Fatalf("artifact = %q, want DONE", got)
	}
}

func TestRealTmux_Interactive_Escalate_FailsFastWithReport(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Please log in to continue", "", "mid", "persist")
	sess := itSession("iesc")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(120*time.Millisecond),
		interactiveLaunch("claude-tmux", sess, launch+" x", itMarker, false))
	if code != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d (ExitUnknownPrompt — escalate must fail fast)", code, ExitUnknownPrompt)
	}
	if _, err := os.Stat(filepath.Join(cfg.Workspace, "escalation-report.json")); err != nil {
		t.Fatalf("escalation report should be written for operator repair: %v", err)
	}
}

func TestRealTmux_Interactive_ArtifactTimeoutOverride(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	cfg.ArtifactTimeoutS = 2
	launchCmd := writeFakeREPL(t, cfg.Worktree, "artifact-timeout", "OVR-READY")
	sess := itSession("ovrto")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(100*time.Millisecond),
		itLaunch(sess, launchCmd, "OVR-READY", 0, false))
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout under the 2s override)", code, ExitArtifactTimeout)
	}
}

func TestRealTmux_Interactive_StuckAutoRespond_TripsLoopGuard(t *testing.T) {
	t.Parallel()
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Warning: this model is deprecated. Continue?", "", "mid", "persist")
	sess := itSession("iloop")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(120*time.Millisecond),
		interactiveLaunch("claude-tmux", sess, launch+" x", itMarker, false))
	if code != ExitRespondLoopGuard {
		t.Fatalf("exit = %d, want %d (ExitRespondLoopGuard — a stuck prompt must not loop forever)", code, ExitRespondLoopGuard)
	}
}
