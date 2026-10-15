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
	wd, _ := os.Getwd()
	cfg := &Config{
		Model: "haiku", Agent: "claude-tmux", Worktree: wd, Workspace: ws,
		PromptFile:       promptFile,
		Artifact:         filepath.Join(root, "artifact"),
		StdoutLog:        filepath.Join(ws, "stdout.log"),
		StderrLog:        filepath.Join(ws, "stderr.log"),
		ArtifactTimeoutS: 45,
	}
	sess := fmt.Sprintf("evolve-bridge-liveint-%d", os.Getpid())
	defer func() { _ = execTmux{}.KillSession(context.Background(), sess) }()

	deps := Deps{Tmux: execTmux{}, Sleep: time.Sleep, Now: time.Now}.withDefaults()
	lp := tmuxLaunch{
		name: sp.name, session: sess, launchCmd: sp.launchCmd,
		promptMarker: sp.marker, inputLineMarker: sp.marker, bootScrollback: sp.bootScrollback,
		bootIntervalS: sp.bootIntervalS, tickDuringBoot: sp.tickDuringBoot,
		exitSeq: sp.exitSeq,
	}
	if _, err := runTmuxREPL(context.Background(), cfg, deps, lp); err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}

	scrollback := readFile(t, filepath.Join(ws, "tmux-final-scrollback.txt"))
	if !strings.Contains(scrollback, "User answered") {
		t.Fatalf("auto-responder did not answer the menu — REPL hung on human input.\n"+
			"scrollback (tail):\n%s", lastLines(stripANSI(scrollback), 25))
	}
}

func TestLiveCLI_Interactive_ClaudeSingleSelect_AutoAnswered(t *testing.T) {
	t.Parallel()
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE")
	assertMenuAutoAnswered(t,
		"IMMEDIATELY use the AskUserQuestion tool to ask me to choose my favorite among "+
			"exactly three options: Alpha, Beta, Gamma. Do nothing else first; just call "+
			"AskUserQuestion right now.")
}

func TestLiveCLI_Interactive_ClaudeMultiSelect_AutoAnswered(t *testing.T) {
	t.Parallel()
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI_INTERACTIVE")
	assertMenuAutoAnswered(t,
		"IMMEDIATELY use the AskUserQuestion tool with multiSelect enabled to ask me to pick "+
			"any of exactly three toppings: Cheese, Mushroom, Onion. Do nothing else first; just "+
			"call AskUserQuestion right now.")
}
