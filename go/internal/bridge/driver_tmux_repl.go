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

// errWorktreeRequired is the CB.2 typed refusal: under a fleet supervisor
// (EVOLVE_FLEET=1) a launch with no explicit worktree must fail closed —
// the cwd fallback would run the agent over another run's tree (or main).
var errWorktreeRequired = errors.New("fleet mode: explicit worktree required (refusing process-cwd fallback)")

// driver_tmux_repl.go — the shared REPL state machine for every *-tmux
// driver (claude-tmux, codex-tmux, agy-tmux). Template Method: the fixed
// flow (spawn session → cd → launch → wait for prompt marker → paste
// prompt → wait for artifact → capture scrollback → exit) lives here; a
// thin per-driver Launch builds a tmuxLaunch spec and runs preflight.
//
// This collapses ~600 lines of near-identical bash (drivers/*-tmux.sh)
// into one engine + three small specs.

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
	name          string // log prefix, e.g. "claude-tmux"
	session       string // resolved tmux session name
	named         bool   // resume-eligible: skip kill + skip exit seq
	launchCmd     string // REPL launch command line
	modelDispatch modelDispatch
	promptMarker  string // boot-ready marker to grep the pane for
	// inputLineMarker locates the LIVE INPUT LINE: text after its LAST
	// occurrence is what has been typed but not yet submitted. Deliberately
	// DISTINCT from promptMarker, which only answers "has the REPL booted" —
	// for agy that is the footer hint "? for shortcuts", which says nothing
	// about where input begins (cycle-1526 audit). Empty means this family
	// declares NO input-line marker: submit-verify then refuses to guess and
	// says so on stderr, rather than anchoring a re-send on a footer and
	// submitting whatever the agent typed.
	inputLineMarker string
	bootScrollback  int       // capture-pane scrollback during boot (0=visible; 200 for alt-screen CLIs)
	bootIntervalS   int       // seconds per boot poll iteration
	tickDuringBoot  bool      // run the auto-respond engine during boot wait (codex/agy: trust prompts)
	bootMenuSkip    string    // non-empty: keypress sent when an interstitial update menu is detected
	exitSeq         []tmuxKey // keystrokes to close the REPL cleanly
	bootOnly        bool      // boot smoke-test: return ExitOK once the marker appears; no prompt/artifact
	// guardDeadShell arms the cycle-274 dead-shell checks (boot rejection +
	// post-paste spill fast-fail). Set by the REAL CLI drivers — their
	// foreground process is never a shell, so a shell pane means the CLI is
	// gone. MUST stay false for harnesses whose "REPL" legitimately IS a
	// shell script (the RealTmux integration fixtures — the PR-71 Ubuntu CI
	// failure this field exists for).
	guardDeadShell bool
}

// launchCmdLine joins an inner-CLI binary with its realized launch flags
// (ADR-0022) into the single REPL launch command line. The flags are the
// per-CLI Realization, so the line carries only argv this CLI understands.
// Each token is shell-quoted because SendKeys delivers ONE shell line, not an
// argv slice: agy 1.0.15's --model values are display names with spaces and
// parens ("Gemini 3.1 Pro (High)", cycle-447). shellQuotePOSIX passes
// safe-charset tokens through verbatim, so claude/codex/ollama launch lines
// are byte-identical to the pre-quoting join.
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
	ar := newAutoResponder(lp.name, cfg.Workspace, deps, human, lp.bootScrollback)
	// Echo-veto (cycle-672): tick() strips pane lines that verbatim-echo this
	// session's own delivered prompt before its exhaustion/escalation scans.
	ar.injectedPrompt = resolvedPrompt

	// ADR-0045 I1: interaction telemetry — every injection this launch fires
	// (auto-respond sends, the one-shot nudge) records a typed outcome in
	// <workspace>/<phase>-interactions.ndjson. Recording runs at EVERY
	// EVOLVE_PHASE_RECOVERY stage including `off`: observation is never the
	// kill-switch's business; only corrective ACTIONS gate on the stage.
	phaseName := orDefault(cfg.Agent, lp.name)
	irec := interaction.NewRecorder(cfg.Workspace)
	ar.rec, ar.phase, ar.cycle = irec, phaseName, cfg.Cycle
	// ADR-0045 I3: the AskBroker's KernelAnswerer over THIS dispatch's closed
	// fact set. It answers only facts the agent's own prompt already carried
	// (artifact path, workspace, worktree, cycle) — structurally unable to
	// disclose anything off-list (threat S7). Gated by the same
	// EVOLVE_PHASE_RECOVERY stage as every other corrective ACTION.
	ar.broker = interaction.NewKernelAnswerer(interaction.KernelFacts{
		ArtifactPath: cfg.Artifact,
		Workspace:    cfg.Workspace,
		Worktree:     cfg.Worktree,
		Cycle:        strconv.Itoa(cfg.Cycle),
	})
	ar.brokerStage = recoveryStageFromEnv(deps)
	// ADR-0045 I4: merge ENFORCE-stage promoted auto-respond rules (durable
	// registry under .evolve/instincts/interaction-rules), re-validated against
	// the immutable healthy-pane corpus at load — a rule a new CLI banner now
	// matches is demoted, never fired. Appended AFTER the manifest rules so a
	// promoted rule can never shadow a vetted built-in (first match wins).
	ar.prompts = append(ar.prompts, loadPromotedPrompts(cfg.ProjectRoot)...)
	// R8.2: shadow-stage rules ride along observe-only — their would-fire
	// outcomes are the measured-clean evidence for the I4 enforce flip.
	ar.shadowRules = loadShadowObservers(cfg.ProjectRoot)
	// A send can be in flight on ANY exit path (boot-time trust prompts
	// included) — flush so the last one is never silently dropped.
	defer ar.flushPending()

	admitRelease, code, err := bootTmuxREPL(ctx, cfg, deps, lp, prep, ar)
	if code != ExitOK || err != nil {
		return code, err
	}
	defer admitRelease()

	// --- Boot smoke-test: the REPL booted to its prompt marker. That is the
	// entire signal we want (the bridge can launch this CLI) — exit cleanly
	// without delivering a prompt or waiting for an artifact. The deferred
	// tmuxCleanup captures the final scrollback for the caller to read.
	if lp.bootOnly {
		if !lp.named {
			for _, k := range lp.exitSeq {
				_ = deps.Tmux.SendKeys(ctx, lp.session, k.keys, k.enter)
				if k.pauseS > 0 {
					deps.Sleep(time.Duration(k.pauseS) * time.Second)
				}
			}
		}
		fmt.Fprintf(deps.Stderr, "%s BOOT-SMOKE: REPL booted; exiting without prompt\n", pfx)
		return ExitOK, nil
	}

	artifactBase, code, err := dispatchTmuxPrompt(ctx, cfg, deps, lp, prep, human, phaseName)
	if code != ExitOK || err != nil {
		return code, err
	}

	cursor := newReplInboxCursor(cfg)
	channel := openReplLiveChannel(cfg, deps, lp)
	defer channel.close()
	waitResult, code := (replWaiter{
		ctx: ctx, cfg: cfg, deps: deps, launch: lp,
		prefix: pfx, phaseName: phaseName, resolvedPrompt: resolvedPrompt,
		artifactBase: artifactBase, responder: ar, recorder: irec, cursor: cursor, channel: channel,
	}).wait()
	if code != ExitOK {
		return code, nil
	}

	// --- Capture scrollback: raw → stderr-log, ANSI-stripped → stdout-log.
	raw, capErr := deps.Tmux.CapturePane(ctx, lp.session, artifactScrollback)
	if raw == "" && capErr != nil {
		// Benign-cancel completion (the on-cancel final poll above): the dead
		// ctx cannot fork tmux, so this capture fails — fall back to the
		// freshest pane the wait loop observed (CB.6 lastGoodPane) instead of
		// writing empty logs and losing the forensic record for exactly the
		// teardown this path exists to classify correctly.
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
		for _, k := range lp.exitSeq {
			_ = deps.Tmux.SendKeys(ctx, lp.session, k.keys, k.enter)
			if k.pauseS > 0 {
				deps.Sleep(time.Duration(k.pauseS) * time.Second)
			}
		}
	}
	contract := cfg.Completion
	if contract == "" {
		contract = "artifact"
	}
	fmt.Fprintf(deps.Stderr, "%s DONE: %s completion verdict = SUCCESS\n", pfx, contract)
	return 0, nil
}
