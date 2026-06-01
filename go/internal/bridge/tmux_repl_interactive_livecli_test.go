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
)

// tmux_repl_interactive_livecli_test.go — the ULTIMATE no-hang proof: drive the
// REAL claude binary through a real tmux REPL into a genuine AskUserQuestion
// menu (which --dangerously-skip-permissions does NOT suppress) with the
// production auto-responder active, and confirm the menu is auto-answered and
// the REPL moves PAST it instead of blocking for a human.
//
// What we assert is the auto-reply property itself: after the auto-responder
// acts, claude prints "User answered Claude's questions" and continues. We do
// NOT assert that the model then completes some follow-up task — that is model
// behavior, not the bridge's job, and conflating the two makes the test flaky
// (haiku often treats answering the question as task-complete). A deliberately
// short ArtifactTimeoutS keeps the run fast: the prompt has no artifact step,
// so the bridge will EC81 after the cap, by which point the scrollback already
// proves the menu was answered.
//
// Gated by EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE=1 (real LLM spend). The
// deterministic, no-spend coverage of the same rules is in
// autorespond_decision_test.go + tmux_repl_interactive_test.go; this is the
// real-CLI ground truth behind them.

func claudeLiveSpec(t *testing.T) liveCLISpec {
	t.Helper()
	for _, sp := range liveCLISpecs {
		if sp.name == "claude-tmux" {
			return sp
		}
	}
	t.Fatal("claude-tmux spec missing from liveCLISpecs")
	return liveCLISpec{}
}

// assertMenuAutoAnswered drives real claude to an AskUserQuestion menu and
// asserts the auto-responder submitted it — proving the workflow continues
// without hanging for human input. Reads the final scrollback the deferred
// tmux cleanup captures (written even on the EC81 short-timeout path).
func assertMenuAutoAnswered(t *testing.T, prompt string) {
	t.Helper()
	sp := claudeLiveSpec(t)
	if _, err := exec.LookPath(sp.bin); err != nil {
		t.Skipf("%s not installed", sp.bin)
	}
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	promptFile := filepath.Join(root, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd() // a real, already-trusted dir for claude to start in
	cfg := &Config{
		Model: "haiku", Agent: "claude-tmux", Worktree: wd, Workspace: ws,
		PromptFile: promptFile,
		Artifact:   filepath.Join(root, "artifact"),
		StdoutLog:  filepath.Join(ws, "stdout.log"),
		StderrLog:  filepath.Join(ws, "stderr.log"),
		// The prompt has no artifact step, so the run ends at this cap, not
		// 300s. ~45s comfortably covers boot + menu render + auto-answer +
		// claude's acknowledgement.
		ArtifactTimeoutS: 45,
	}
	sess := fmt.Sprintf("evolve-bridge-liveint-%d", os.Getpid())
	defer func() { _ = execTmux{}.KillSession(context.Background(), sess) }()

	// Real Deps: real tmux, real Sleep, real Now → the auto-responder ticks the
	// artifact-wait loop and answers the menu exactly as in production. Name
	// "claude-tmux" loads the real manifest (askuserquestion_select / _multiselect).
	deps := Deps{Tmux: execTmux{}, Sleep: time.Sleep, Now: time.Now}.withDefaults()
	lp := tmuxLaunch{
		name: sp.name, session: sess, launchCmd: sp.launchCmd,
		promptMarker: sp.marker, bootScrollback: sp.bootScrollback,
		bootIntervalS: sp.bootIntervalS, tickDuringBoot: sp.tickDuringBoot,
		exitSeq: sp.exitSeq,
	}
	if _, err := runTmuxREPL(context.Background(), cfg, deps, lp); err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}

	// The deferred tmuxCleanup captures the final pane to this file even on the
	// EC81 path. "User answered Claude's questions" is claude's confirmation
	// that the AskUserQuestion was submitted → the auto-reply unblocked the REPL.
	scrollback := readFile(t, filepath.Join(ws, "tmux-final-scrollback.txt"))
	if !strings.Contains(scrollback, "User answered") {
		t.Fatalf("auto-responder did not answer the menu — REPL hung on human input.\n"+
			"scrollback (tail):\n%s", lastLines(stripANSI(scrollback), 25))
	}
}

func TestLiveCLI_Interactive_ClaudeSingleSelect_AutoAnswered(t *testing.T) {
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE")
	assertMenuAutoAnswered(t,
		"IMMEDIATELY use the AskUserQuestion tool to ask me to choose my favorite among "+
			"exactly three options: Alpha, Beta, Gamma. Do nothing else first; just call "+
			"AskUserQuestion right now.")
}

func TestLiveCLI_Interactive_ClaudeMultiSelect_AutoAnswered(t *testing.T) {
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE")
	assertMenuAutoAnswered(t,
		"IMMEDIATELY use the AskUserQuestion tool with multiSelect enabled to ask me to pick "+
			"any of exactly three toppings: Cheese, Mushroom, Onion. Do nothing else first; just "+
			"call AskUserQuestion right now.")
}
