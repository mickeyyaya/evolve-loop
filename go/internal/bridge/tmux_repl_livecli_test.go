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

// tmux_repl_livecli_test.go — the REAL-CLI tier of the tmux REPL suite,
// grounded in the ground-truth capture in
// knowledge-base/research/tmux-repl-cli-behavior-2026-05-26.md.
//
// These drive the ACTUAL claude / codex / agy binaries inside a real tmux
// server, so they validate the one thing neither the fake-tmux unit tests
// nor the scripted-fake integration tests can: that the boot markers the
// drivers grep for (❯ / › / "? for shortcuts") actually appear in the real
// CLI's pane, at the real version installed on this host.
//
// Two tiers, both OFF by default (so `go test` / CI needs neither the CLIs
// installed nor any LLM spend):
//
//	EVOLVE_BRIDGE_LIVE_CLI=1            → boot-marker tier (cheap: launches
//	                                      the CLI, asserts the marker, exits.
//	                                      No prompt delivered → no inference).
//	EVOLVE_BRIDGE_LIVE_CLI_ROUNDTRIP=1 → full round-trip tier (real LLM
//	                                      spend: prompt → artifact write).
//
// The specs below MUST match the production driver constants. If a CLI
// upgrade moves a marker, the boot-marker test fails loudly here before it
// breaks a real cycle — that is the point.

type liveCLISpec struct {
	name           string // driver name (also the manifest key for auto-respond)
	bin            string // binary to LookPath
	launchCmd      string // exact REPL launch line
	marker         string // boot-ready marker the driver greps for
	bootScrollback int
	bootIntervalS  int
	tickDuringBoot bool
	exitSeq        []tmuxKey
}

// liveCLISpecs mirrors the production *-tmux drivers + the captured
// ground truth. Keep in sync with driver_claudetmux.go / driver_codextmux.go
// / driver_agytmux.go and the knowledge-base capture doc.
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

// --- boot-marker tier (cheap: no prompt, no inference) --------------------

// TestLiveCLI_BootMarkerDetected validates that each driver's boot marker
// actually appears in the real CLI's pane within the boot budget. This is
// the load-bearing assumption of runTmuxREPL — if it drifts, launches die
// with EC 80. Costs nothing beyond CLI startup (no prompt is delivered, so
// no inference).
//
// It mirrors runTmuxREPL's boot loop EXACTLY, including ticking the
// auto-responder for tickDuringBoot CLIs — codex/agy show a trust prompt at
// boot that must be dismissed with Enter before the ready marker appears.
// Dismissing that dialog is free (it is not inference). Replicating it is
// what makes this cheap tier faithful to the real bridge boot.
func TestLiveCLI_BootMarkerDetected(t *testing.T) {
	liveCLIGate(t, "EVOLVE_BRIDGE_LIVE_CLI")
	wd, _ := os.Getwd() // a real, trusted dir for the CLI to start in
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
			deps := Deps{Tmux: tx, Now: time.Now, Stderr: &stderr}.withDefaults()
			// auto-responder seeded from sp.name's manifest → handles the
			// codex/agy boot trust prompt, same as runTmuxREPL.
			ar := newAutoResponder(sp.name, t.TempDir(), deps, false, 0)

			if err := tx.NewSession(ctx, sess, tmuxPaneWidth, tmuxPaneHeight); err != nil {
				t.Fatalf("new-session: %v", err)
			}
			time.Sleep(time.Second)
			_ = tx.SendKeys(ctx, sess, "cd "+wd, true)
			time.Sleep(time.Second)
			_ = tx.SendKeys(ctx, sess, sp.launchCmd, true)

			// Poll for the marker using the SAME deadline as production
			// (tmuxREPLBootTimeoutS) so a pass here means a pass in a real
			// launch. Captured boot was 1–2s; the budget is the safety
			// ceiling. Mirrors runTmuxREPL's boot loop incl. tickDuringBoot.
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
					ar.tick(ctx, sess) // dismiss trust prompt (free, no inference)
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

// --- full round-trip tier (real LLM spend) --------------------------------

// TestLiveCLI_FullRoundtrip drives a complete runTmuxREPL launch against the
// real CLI: boot → paste a prompt that asks the model to write the artifact
// → artifact-wait → scrollback → exit. Proves the whole bridge contract end
// to end against the live CLI. Gated separately because it spends tokens.
func TestLiveCLI_FullRoundtrip(t *testing.T) {
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
			// $ARTIFACT_PATH is substituted by preparePrompt → cfg.Artifact.
			prompt := "Use your file-writing tool to write exactly the single " +
				"word PONG (uppercase, no other text) to the file $ARTIFACT_PATH. Then stop."
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

			deps := Deps{Tmux: tx, Sleep: time.Sleep, Now: time.Now}.withDefaults()
			lp := tmuxLaunch{
				name: sp.name, session: sess, launchCmd: sp.launchCmd,
				promptMarker: sp.marker, bootScrollback: sp.bootScrollback,
				bootIntervalS: sp.bootIntervalS, tickDuringBoot: sp.tickDuringBoot,
				exitSeq: sp.exitSeq,
			}
			code, err := runTmuxREPL(ctx, cfg, deps, lp)
			if err != nil {
				t.Fatalf("runTmuxREPL err: %v", err)
			}
			if code != ExitOK {
				t.Fatalf("%s round-trip exit = %d, want ExitOK", sp.name, code)
			}
			got, rerr := os.ReadFile(cfg.Artifact)
			if rerr != nil || !strings.Contains(string(got), "PONG") {
				t.Fatalf("%s artifact = %q (err=%v), want it to contain PONG", sp.name, string(got), rerr)
			}
		})
	}
}
