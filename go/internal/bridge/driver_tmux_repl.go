package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// errWorktreeRequired refuses to fall back to the process cwd under a fleet
// supervisor (EVOLVE_FLEET=1): that fallback would run the agent over
// another run's worktree, or main.
var errWorktreeRequired = errors.New("fleet mode: explicit worktree required (refusing process-cwd fallback)")

const (
	tmuxPromptMarkerDefault  = "❯" // claude REPL marker (codex ›, agy "? for shortcuts")
	tmuxREPLBootTimeoutS     = 60  // boot-wait deadline (poll loop)
	tmuxArtifactTimeoutS     = 300 // artifact-wait deadline (poll @ 2s)
	transientRedispatchDelay = 15 * time.Second
	tmuxPaneWidth            = 220
	tmuxPaneHeight           = 80
	tmuxArtifactScrollback   = 10000 // deep capture for final scrollback

	// maxInjectDefer bounds how many times a mid-turn command is re-queued
	// while the agent is busy, so a never-idle agent cannot loop forever.
	maxInjectDefer = 10
	// injectInterruptSettle is the pause after an ESC before injecting text.
	injectInterruptSettle = 500 * time.Millisecond
)

// tmuxKey is one keystroke group sent to the REPL (e.g. {"/exit", true, 2}
// = send "/exit" + Enter, then sleep 2s). Used for the per-driver exit seq.
type tmuxKey struct {
	keys   string
	enter  bool
	pauseS int
}

// tmuxLaunch is the per-driver spec the shared engine runs. Everything
// driver-specific that the state machine needs is captured here; the
// driver computes it after its own preflight.
type tmuxLaunch struct {
	name            string // log prefix, e.g. "claude-tmux"
	session         string // resolved tmux session name
	named           bool   // resume-eligible: skip kill + skip exit seq
	launchCmd       string // REPL launch command line
	modelDispatch   modelDispatch
	promptMarker    string    // boot-ready marker to grep the pane for
	inputLineMarker string    // live-input-line marker; empty when the family has none
	bootScrollback  int       // capture-pane scrollback during boot (0=visible; 200 for alt-screen CLIs)
	bootIntervalS   int       // seconds per boot poll iteration
	tickDuringBoot  bool      // run the auto-respond engine during boot wait (codex/agy: trust prompts)
	exitSeq         []tmuxKey // keystrokes to close the REPL cleanly
	bootOnly        bool      // boot smoke-test: return ExitOK once the marker appears; no prompt/artifact
	guardDeadShell  bool      // true for real CLI drivers; false for shell-script REPL test harnesses
	modelCheck      launchModelCheck
}

func launchCmdLine(binary string, flags []string) string {
	if len(flags) == 0 {
		return binary
	}
	quoted := make([]string, len(flags))
	for i, f := range flags {
		quoted[i] = shellQuotePOSIX(f)
	}
	return binary + " " + strings.Join(quoted, " ")
}

// runTmuxREPL drives the shared interactive-REPL flow and returns a bridge
// exit code. Preconditions (gate, cost guards, model/session resolution)
// are the driver's responsibility before calling this.
func runTmuxREPL(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch) (int, error) {
	prep, code, err := prepareTmuxREPL(ctx, cfg, deps, lp)
	if code != ExitOK || err != nil {
		return code, err
	}
	if prep.namedExists {
		observeModelDispatch(deps, modelDispatch{source: modelDispatchResumed})
	}
	pfx := prep.prefix
	resolvedPrompt := prep.resolvedPrompt
	scrollbackFile := prep.scrollbackFile
	artifactScrollback := prep.artifactScrollback
	defer tmuxCleanup(ctx, deps, lp.name, lp.session, scrollbackFile, lp.named, artifactScrollback)

	// Auto-respond fallback engine, seeded from the CLI's manifest rules.
	human := humanActive(deps, cfg.HumanInput)
	ar := newLaunchAutoResponder(cfg.Workspace, deps, lp, human)
	// tick() strips pane lines that verbatim-echo this session's own
	// delivered prompt before its exhaustion/escalation scans.
	ar.injectedPrompt = resolvedPrompt

	phaseName := orDefault(cfg.Agent, lp.name)
	irec := interaction.NewRecorder(cfg.Workspace)
	ar.rec, ar.phase, ar.cycle = irec, phaseName, cfg.Cycle
	ar.broker = interaction.NewKernelAnswerer(interaction.KernelFacts{
		ArtifactPath: cfg.Artifact,
		Workspace:    cfg.Workspace,
		Worktree:     cfg.Worktree,
		Cycle:        strconv.Itoa(cfg.Cycle),
	})
	ar.brokerStage = recoveryStageFromEnv(deps)
	// Promoted rules append after the manifest rules so a promoted rule can
	// never shadow a vetted built-in (first match wins).
	ar.prompts = append(ar.prompts, loadPromotedPrompts(cfg.ProjectRoot)...)
	// Shadow-stage rules ride along observe-only, measuring whether a
	// promotion to enforce would be safe.
	ar.shadowRules = loadShadowObservers(cfg.ProjectRoot)
	// A send can be in flight on ANY exit path (boot-time trust prompts
	// included) — flush so the last one is never silently dropped.
	defer ar.flushPending()

	admitRelease, code, err := bootTmuxREPL(ctx, cfg, deps, lp, prep, ar)
	if code != ExitOK || err != nil {
		return code, err
	}
	defer admitRelease()

	if code := lp.verifyBootedModel(ctx, bootedModelRun{cfg: cfg, deps: deps, responder: ar}); code != ExitOK {
		return code, nil
	}
	if lp.bootOnly {
		return lp.endBootSmoke(ctx, deps, pfx), nil
	}

	dispatchBase, paste, code, err := dispatchTmuxPrompt(ctx, cfg, deps, lp, prep, human, phaseName)
	if code != ExitOK || err != nil {
		return code, err
	}

	cursor := newReplInboxCursor(cfg)
	channel := openReplLiveChannel(cfg, deps, lp)
	defer channel.close()
	waitResult, code := (replWaiter{
		ctx: ctx, cfg: cfg, deps: deps, launch: lp,
		prefix: pfx, phaseName: phaseName, resolvedPrompt: resolvedPrompt,
		dispatchBase: dispatchBase, paste: paste, responder: ar, recorder: irec, cursor: cursor, channel: channel,
	}).wait()
	if code != ExitOK {
		return code, nil
	}

	raw, capErr := deps.Tmux.CapturePane(ctx, lp.session, artifactScrollback)
	if raw == "" && capErr != nil {
		raw = waitResult.lastGoodPane
	}
	waitResult.recordTokens(raw)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
	writeTokenUsage(cfg.Workspace, waitResult.peakTokens)
	fmt.Fprintf(deps.Stderr, "%s scrollback captured\n", pfx)

	switch {
	case lp.named:
		fmt.Fprintf(deps.Stderr, "%s RESUME-PRESERVE: skipping exit; REPL stays running for next launch\n", pfx)
	case ctx.Err() != nil:
		// Dead ctx: SendKeys is a guaranteed no-op, so the exit sequence would
		// only burn its inter-key pauses. The deferred session kill (and the
		// cycle-start orphan GC behind it) owns teardown here.
		fmt.Fprintf(deps.Stderr, "%s exit sequence skipped (ctx done) — session kill handles teardown\n", pfx)
	default:
		lp.sendExitSeq(ctx, deps)
	}
	contract := completionContractName(cfg.Completion)
	fmt.Fprintf(deps.Stderr, "%s DONE: %s completion verdict = SUCCESS\n", pfx, contract)
	return 0, nil
}

func (lp tmuxLaunch) endBootSmoke(ctx context.Context, deps Deps, pfx string) int {
	if !lp.named {
		lp.sendExitSeq(ctx, deps)
	}
	fmt.Fprintf(deps.Stderr, "%s BOOT-SMOKE: REPL booted; exiting without prompt\n", pfx)
	return ExitOK
}

func (lp tmuxLaunch) sendExitSeq(ctx context.Context, deps Deps) {
	for _, k := range lp.exitSeq {
		_ = deps.Tmux.SendKeys(ctx, lp.session, k.keys, k.enter)
		if k.pauseS > 0 {
			deps.Sleep(time.Duration(k.pauseS) * time.Second)
		}
	}
}
