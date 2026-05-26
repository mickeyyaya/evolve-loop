package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tmux_repl_interactive_test.go — proves the AUTO-REPLY POLICY keeps the
// workflow flowing on a REAL tmux server with the REAL embedded manifest
// rules. Unlike driver_claudetmux_test.go (fake TmuxController) this runs the
// full runTmuxREPL flow against execTmux, so the auto-responder's capture-pane
// → regex-match → send-keys round-trip is exercised with real tmux I/O.
//
// The fake CLI is the linchpin: it emits a real interactive-prompt string and
// ONLY writes the artifact once it has received the auto-responder's expected
// keystroke. So a green test proves the policy actually unblocked the REPL; a
// failed auto-reply would mean the fake never gets the key → no artifact →
// EC81 artifact-timeout → loud failure. That is the "no hang on human input"
// guarantee, end to end.
//
// Escalate/loop-guard scenarios use "persist" mode: the fake emits the prompt
// and never unblocks, so the auto-responder's own EC85/EC86 (not an artifact)
// must end the run — proving the bridge fails FAST instead of hanging.

// writeInteractiveFake writes a per-scenario fake CLI. Everything is BAKED
// into the script body (never argv) so tmux's echo of the launch command can't
// leak the marker or prompt text into the pane and trip a false match — the
// same discipline writeFakeREPL documents.
//
//	timing "boot" → emit the prompt BEFORE the marker (trust dialogs; needs
//	                tickDuringBoot). "mid" → emit after the prompt is delivered.
//	mode   "unblock" → wait for `expect` then continue. "persist" → never
//	                unblock (escalate / loop-guard; runTmuxREPL returns on its own).
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
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write interactive fake: %v", err)
	}
	return path // no args: all behavior is baked in
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

// interactiveConfig wires the prompt file to carry "ARTIFACT=$ARTIFACT_PATH";
// preparePrompt substitutes the real artifact path, and the fake echoes it
// back into a real on-disk write the artifact-wait loop observes.
func interactiveConfig(t *testing.T) *Config {
	t.Helper()
	cfg := itConfig(t, "x")
	mustWrite(t, cfg.PromptFile, "ARTIFACT=$ARTIFACT_PATH")
	return cfg
}

const itMarker = "ITREADY"

// --- 1. claude single-select AskUserQuestion → bare Enter unblocks ----------

func TestRealTmux_Interactive_ClaudeSingleSelect_AutoEnter(t *testing.T) {
	requireTmux(t)
	cfg := interactiveConfig(t)
	// Footer matches askuserquestion_select; no "[ ]" so the multiselect rule
	// does not fire. Auto-reply is a bare Enter (response_keys "Enter").
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

// --- 1b. claude multi-select → "Enter,Right,Enter" sequence unblocks --------

func TestRealTmux_Interactive_ClaudeMultiSelect_AutoSubmit(t *testing.T) {
	requireTmux(t)
	cfg := interactiveConfig(t)
	// Prompt carries the highlighted checkbox row → matches askuserquestion_
	// multiselect (response "Enter,Right,Enter"). The fake unblocks only after
	// the full two-Enter sequence reaches it through real tmux send-keys, so a
	// green run proves the multi-keystroke delivery (incl. the "Right" named
	// key) end to end — the deterministic counterpart to the live-CLI proof.
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

// --- 2. codex trust dialog at boot → "1,Enter" unblocks ---------------------

func TestRealTmux_Interactive_CodexTrust_AutoRespondAtBoot(t *testing.T) {
	requireTmux(t)
	cfg := interactiveConfig(t)
	launch := writeInteractiveFake(t, cfg.Worktree, itMarker,
		"Do you trust the contents of this directory?", "1", "boot", "unblock")
	sess := itSession("itrust")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	// tickDuringBoot=true: the codex/agy boot-time trust path.
	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(250*time.Millisecond),
		interactiveLaunch("codex-tmux", sess, launch+" x", itMarker, true))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK (trust dialog should be auto-dismissed at boot)", code)
	}
	if got := readFile(t, cfg.Artifact); got != "DONE" {
		t.Fatalf("artifact = %q, want DONE (boot trust not dismissed → workflow stalled)", got)
	}
}

// --- 3. agy permission prompt → "y,Enter" unblocks --------------------------

func TestRealTmux_Interactive_AgyPermission_AutoYes(t *testing.T) {
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

// --- 4. escalate prompt → EC85 fast, no hang, escalation report written -----

func TestRealTmux_Interactive_Escalate_FailsFastWithReport(t *testing.T) {
	requireTmux(t)
	cfg := interactiveConfig(t)
	// auth_recheck is policy=escalate → the bridge cannot self-resolve OAuth,
	// so it must abandon FAST (EC85) rather than hang for a human.
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

// --- 5b. Config.ArtifactTimeoutS overrides the 300s default deadline --------

func TestRealTmux_Interactive_ArtifactTimeoutOverride(t *testing.T) {
	requireTmux(t)
	// The fake boots but never writes the artifact; ArtifactTimeoutS=2 (not the
	// 300s default) must bound the wait → fast EC81. Guards the per-launch
	// timeout seam the live auto-reply tests rely on.
	cfg := itConfig(t, "ARTIFACT=/dev/null")
	cfg.ArtifactTimeoutS = 2
	launchCmd := writeFakeREPL(t, cfg.Worktree, "artifact-timeout", "OVR-READY")
	sess := itSession("ovrto")
	defer itTmuxCtl.KillSession(context.Background(), sess)

	code, _ := runTmuxREPL(context.Background(), cfg, itDeps(10*time.Millisecond),
		itLaunch(sess, launchCmd, "OVR-READY", 0, false))
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout under the 2s override)", code, ExitArtifactTimeout)
	}
}

// --- 6. stuck auto_respond prompt → loop guard EC86, no infinite hang -------

func TestRealTmux_Interactive_StuckAutoRespond_TripsLoopGuard(t *testing.T) {
	requireTmux(t)
	cfg := interactiveConfig(t)
	// model_deprecation is auto_respond (y,Enter), but the fake never clears
	// it: the engine re-sends each tick and the >5× loop guard must abandon.
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
