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

type liveCLISpec struct {
	name           string
	bin            string
	launchCmd      string
	marker         string
	bootScrollback int
	bootIntervalS  int
	tickDuringBoot bool
	exitSeq        []tmuxKey
}

var liveCLISpecs = []liveCLISpec{
	{
		name: "claude-tmux", bin: "claude",
		launchCmd: "claude --model haiku --dangerously-skip-permissions",
		marker:    "❯", bootScrollback: 0, bootIntervalS: 1, tickDuringBoot: true,
		exitSeq: []tmuxKey{{keys: "/exit", enter: true, pauseS: 1}},
	},
	{
		name: "codex-tmux", bin: "codex",
		launchCmd: "codex",
		marker:    "›", bootScrollback: 200, bootIntervalS: 2, tickDuringBoot: true,
		exitSeq: []tmuxKey{{keys: "/quit", enter: true, pauseS: 1}},
	},
	{
		name: "agy-tmux", bin: "agy",
		launchCmd: "agy --dangerously-skip-permissions",
		marker:    "? for shortcuts", bootScrollback: 200, bootIntervalS: 2, tickDuringBoot: true,
		exitSeq: []tmuxKey{{keys: "C-c", enter: false, pauseS: 1}, {keys: "C-c", enter: false, pauseS: 1}},
	},
}

func liveCLIGate(t *testing.T, envKey string) {
	t.Helper()
	if os.Getenv(envKey) != "1" {
		t.Skipf("%s != 1; skipping real-CLI tier (no binaries / no LLM spend in default runs)", envKey)
	}
	requireTmux(t)
}

func TestLiveCLI_BootMarkerDetected(t *testing.T) {
	t.Parallel()
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI")
	wd, _ := os.Getwd()
	tx := execTmux{}
	ctx := context.Background()

	for _, sp := range liveCLISpecs {
		sp := sp
		t.Run(sp.name, func(t *testing.T) {
			if _, err := exec.LookPath(sp.bin); err != nil {
				t.Skipf("%s not installed", sp.bin)
			}
			sess := fmt.Sprintf("evolve-bridge-livemk-%s-%d", sp.bin, os.Getpid())
			defer func() { _ = tx.KillSession(ctx, sess) }()

			var stderr strings.Builder
			deps := Deps{Tmux: tx, Now: time.Now, Stderr: &stderr, SignalProcess: refuseTestSignal}.withDefaults()
			ar := newAutoResponder(sp.name, t.TempDir(), deps, false, 0)

			if err := tx.NewSession(ctx, sess, tmuxPaneWidth, tmuxPaneHeight); err != nil {
				t.Fatalf("new-session: %v", err)
			}
			time.Sleep(time.Second)
			_ = tx.SendKeys(ctx, sess, "cd "+wd, true)
			time.Sleep(time.Second)
			_ = tx.SendKeys(ctx, sess, sp.launchCmd, true)

			const budget = tmuxREPLBootTimeoutS
			interval := sp.bootIntervalS
			if interval <= 0 {
				interval = 1
			}
			found := false
			for elapsed := 0; elapsed < budget; elapsed += interval {
				time.Sleep(time.Duration(interval) * time.Second)
				pane, _ := tx.CapturePane(ctx, sess, sp.bootScrollback)
				if strings.Contains(pane, sp.marker) {
					found = true
					break
				}
				if sp.tickDuringBoot {
					ar.tick(ctx, sess)
				}
			}
			if !found {
				pane, _ := tx.CapturePane(ctx, sess, 200)
				t.Fatalf("boot marker %q never appeared for %s within %ds.\n"+
					"The marker constant has likely drifted from the installed CLI version.\n"+
					"--- pane ---\n%s", sp.marker, sp.name, budget, pane)
			}
		})
	}
}

func TestLiveCLI_FullRoundtrip(t *testing.T) {
	t.Parallel()
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI_ROUNDTRIP")
	wd, _ := os.Getwd()
	tx := execTmux{}
	ctx := context.Background()

	for _, sp := range liveCLISpecs {
		sp := sp
		t.Run(sp.name, func(t *testing.T) {
			if _, err := exec.LookPath(sp.bin); err != nil {
				t.Skipf("%s not installed", sp.bin)
			}
			root := t.TempDir()
			ws := filepath.Join(root, "ws")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			promptFile := filepath.Join(root, "prompt.txt")
			head, middle, tail := "head-"+filepath.Base(root), "middle-台灣", "tail-λ"
			want := head + "|" + middle + "|" + tail
			padding := strings.Repeat("PADDING: benign transport fixture text, no task or instructions.\n", 400)
			prompt := "Read DELIVERY_HEAD, DELIVERY_MIDDLE and DELIVERY_TAIL only from this message. " +
				"Use shell printf with format %s (not echo) to write their values joined by | exactly, with no newline, " +
				"to $ARTIFACT_PATH. Do not read prompt files or source files. Ignore PADDING lines. " +
				"This is the complete task; no research or other work is needed.\n" +
				"DELIVERY_HEAD=" + head + "\n" + padding +
				"DELIVERY_MIDDLE=" + middle + "\n" + padding +
				"DELIVERY_TAIL=" + tail + "\nThen stop after writing the file."
			if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := &Config{
				Model: "haiku", Agent: sp.name, Worktree: wd, Workspace: ws,
				PromptFile: promptFile,
				Artifact:   filepath.Join(root, "artifact"),
				StdoutLog:  filepath.Join(ws, "stdout.log"),
				StderrLog:  filepath.Join(ws, "stderr.log"),
			}
			sess := fmt.Sprintf("evolve-bridge-livert-%s-%d", sp.bin, os.Getpid())
			defer func() { _ = tx.KillSession(ctx, sess) }()

			deps := Deps{Tmux: tx, Sleep: time.Sleep, Now: time.Now, SignalProcess: refuseTestSignal}.withDefaults()
			lp := tmuxLaunch{
				name: sp.name, session: sess, launchCmd: sp.launchCmd,
				promptMarker: sp.marker, inputLineMarker: sp.marker, bootScrollback: sp.bootScrollback,
				bootIntervalS: sp.bootIntervalS, tickDuringBoot: sp.tickDuringBoot,
				exitSeq: sp.exitSeq,
			}
			runCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			code, err := runTmuxREPL(runCtx, cfg, deps, lp)
			if err != nil {
				t.Fatalf("runTmuxREPL err: %v", err)
			}
			if code != ExitOK {
				t.Fatalf("%s round-trip exit = %d, want ExitOK", sp.name, code)
			}
			got, rerr := os.ReadFile(cfg.Artifact)
			if rerr != nil || string(got) != want {
				t.Fatalf("%s artifact = %q (err=%v), want exact head/middle/tail %q", sp.name, string(got), rerr, want)
			}
		})
	}
}
