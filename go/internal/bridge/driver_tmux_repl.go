package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/keyspec"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// errWorktreeRequired is the CB.2 typed refusal: under a fleet supervisor
// (EVOLVE_FLEET=1) a launch with no explicit worktree must fail closed —
// the cwd fallback would run the agent over another run's tree (or main).
var errWorktreeRequired = errors.New("fleet mode: explicit worktree required (refusing process-cwd fallback)")

// emitChannelBreadcrumb writes one structured channel marker to w. The producer's
// correlator parses these to bracket an injected ask's answer span (ADR-0037).
// Empty corrID is a no-op so non-correlated injects add no noise. The caller
// chooses w: the <agent>-breadcrumbs.live file when EVOLVE_CHANNEL=1, else
// io.Discard (the producer tails the FILE — RT2 moved these off the in-memory
// stderr stream a discarded producer never read).
func emitChannelBreadcrumb(w io.Writer, channel, corrID string) {
	if corrID == "" {
		return
	}
	fmt.Fprintf(w, "{\"evolve_channel\":%q,\"corr_id\":%q}\n", channel, corrID)
}

// channelEnabled reports whether the live bidirectional channel (ADR-0037) is
// on. ADR-0045 I6 folded the rollout into EVOLVE_PHASE_RECOVERY: the channel is
// implied by the stage (enforce → on; off/shadow → off, byte-identical), and
// the legacy EVOLVE_CHANNEL flag is deprecated — honored one release with a
// one-time WARN. channel.Enabled is the single source for both this driver and
// the observer adapter.
func channelEnabled(deps Deps) bool {
	explicit, _ := lookupEnv(deps, "EVOLVE_CHANNEL")
	on, deprecated := channel.Enabled(recoveryStageFromEnv(deps), explicit)
	if deprecated {
		fmt.Fprintf(deps.Stderr, "[bridge] WARN: EVOLVE_CHANNEL is deprecated and will be removed next release — the live channel now rides EVOLVE_PHASE_RECOVERY (enforce implies it). See docs/architecture/control-flags.md.\n")
	}
	return on
}

// paneProfileFor resolves the panestream PaneProfile for a tmux driver by
// stripping the "-tmux" suffix from the driver name (claude-tmux → claude). An
// unknown driver (e.g. the test "itest-tmux") falls back to a profile built
// from the launch's own prompt marker so the delta extractor still has a
// content boundary.
func paneProfileFor(lp tmuxLaunch) panestream.PaneProfile {
	cli := strings.TrimSuffix(lp.name, "-tmux")
	if p, ok := panestream.Profiles[cli]; ok {
		return p
	}
	return panestream.PaneProfile{Name: cli, BoundaryMarker: lp.promptMarker}
}

// driver_tmux_repl.go — the shared REPL state machine for every *-tmux
// driver (claude-tmux, codex-tmux, agy-tmux). Template Method: the fixed
// flow (spawn session → cd → launch → wait for prompt marker → paste
// prompt → wait for artifact → capture scrollback → exit) lives here; a
// thin per-driver Launch builds a tmuxLaunch spec and runs preflight.
//
// This collapses ~600 lines of near-identical bash (drivers/*-tmux.sh)
// into one engine + three small specs.

const (
	tmuxPromptMarkerDefault = "❯" // claude REPL marker (codex ›, agy "? for shortcuts")
	tmuxREPLBootTimeoutS    = 60  // boot-wait deadline (poll loop)
	tmuxArtifactTimeoutS    = 300 // artifact-wait deadline (poll @ 2s)
	tmuxPaneWidth           = 220
	tmuxPaneHeight          = 80
	tmuxArtifactScrollback  = 10000 // deep capture for final scrollback

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
	name           string    // log prefix, e.g. "claude-tmux"
	session        string    // resolved tmux session name
	named          bool      // resume-eligible: skip kill + skip exit seq
	launchCmd      string    // REPL launch command line
	promptMarker   string    // boot-ready marker to grep the pane for
	bootScrollback int       // capture-pane scrollback during boot (0=visible; 200 for alt-screen CLIs)
	bootIntervalS  int       // seconds per boot poll iteration
	tickDuringBoot bool      // run the auto-respond engine during boot wait (codex/agy: trust prompts)
	bootMenuSkip   string    // non-empty: keypress sent when an interstitial update menu is detected
	exitSeq        []tmuxKey // keystrokes to close the REPL cleanly
	bootOnly       bool      // boot smoke-test: return ExitOK once the marker appears; no prompt/artifact
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
func launchCmdLine(binary string, flags []string) string {
	if len(flags) == 0 {
		return binary
	}
	return binary + " " + strings.Join(flags, " ")
}

// runTmuxREPL drives the shared interactive-REPL flow and returns a bridge
// exit code. Preconditions (gate, cost guards, model/session resolution)
// are the driver's responsibility before calling this.
func runTmuxREPL(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch) (int, error) {
	pfx := "[" + lp.name + "]"

	workingDir := cfg.Worktree
	if workingDir == "" {
		// CB.2: the cwd fallback launches the agent over WHATEVER directory
		// the dispatching process sits in. Under a fleet supervisor that may
		// be another run's tree — fail closed (typed, exit 10 is a non-trigger
		// code: a config bug must surface, never CLI-fallback). Single mode
		// keeps the fallback for operator ergonomics, but loudly.
		if v, _ := lookupEnv(deps, "EVOLVE_FLEET"); envchain.BoolValue(v, false) {
			return ExitBadFlags, fmt.Errorf("%s %w", pfx, errWorktreeRequired)
		}
		workingDir, _ = os.Getwd()
		fmt.Fprintf(deps.Stderr, "%s WARN no worktree designated — falling back to process cwd %s (single-driver mode only; fleet mode refuses this)\n", pfx, workingDir)
	}
	if !isDir(workingDir) {
		fmt.Fprintf(deps.Stderr, "%s working dir does not exist: %s\n", pfx, workingDir)
		return ExitBadFlags, nil
	}

	if err := ensureDirs(cfg); err != nil {
		return ExitBadFlags, fmt.Errorf("%s %w", pfx, err)
	}
	// Boot smoke-test (lp.bootOnly) skips prompt prep entirely — it never
	// delivers a task, so there is no prompt to resolve/write.
	var resolvedPromptFile string
	if !lp.bootOnly {
		prompt, err := preparePrompt(cfg, deps)
		if err != nil {
			return ExitBadFlags, err
		}
		resolvedPromptFile = filepath.Join(cfg.Workspace, "resolved-prompt.txt")
		if err := os.WriteFile(resolvedPromptFile, []byte(prompt+"\n"), 0o644); err != nil {
			return ExitBadFlags, fmt.Errorf("%s write resolved prompt: %w", pfx, err)
		}
	}

	namedExists := false
	if lp.named {
		if deps.Tmux.HasSession(ctx, lp.session) {
			namedExists = true
			fmt.Fprintf(deps.Stderr, "%s RESUME: reattaching to existing named session '%s'\n", pfx, lp.session)
		} else {
			fmt.Fprintf(deps.Stderr, "%s CREATE-NAMED: new named session '%s' (persists on exit for resume)\n", pfx, lp.session)
		}
	}
	scrollbackFile := filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")
	artifactScrollback := envInt(deps, "EVOLVE_SCROLLBACK_LINES", tmuxArtifactScrollback)
	fmt.Fprintf(deps.Stderr, "%s session=%s model=%s workdir=%s\n", pfx, lp.session, cfg.Model, workingDir)
	defer tmuxCleanup(ctx, deps, lp.name, lp.session, scrollbackFile, lp.named, artifactScrollback)

	// Auto-respond fallback engine, seeded from the CLI's manifest rules.
	human := humanActive(deps, cfg.HumanInput)
	ar := newAutoResponder(lp.name, cfg.Workspace, deps, human, lp.bootScrollback)

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

	// --- Spawn + cd + launch + wait for the REPL prompt marker.
	if !namedExists {
		// CB.2: bind the pane cwd at session birth when the controller can
		// (`tmux new-session -c`); the cd keystroke below stays as the second
		// layer for capability-less controllers. (A RESUMED named session gets
		// neither — it deliberately keeps whatever state it was left in.)
		if ws, ok := deps.Tmux.(workdirSessionStarter); ok {
			if err := ws.NewSessionIn(ctx, lp.session, tmuxPaneWidth, tmuxPaneHeight, workingDir); err != nil {
				return ExitBadFlags, fmt.Errorf("%s new-session: %w", pfx, err)
			}
		} else if err := deps.Tmux.NewSession(ctx, lp.session, tmuxPaneWidth, tmuxPaneHeight); err != nil {
			return ExitBadFlags, fmt.Errorf("%s new-session: %w", pfx, err)
		}
		deps.Sleep(time.Second)
		_ = deps.Tmux.SendKeys(ctx, lp.session, "cd "+workingDir, true)
		deps.Sleep(time.Second)
		launchCmd := lp.launchCmd
		if len(cfg.ExtraFlags) > 0 {
			launchCmd += " " + strings.Join(cfg.ExtraFlags, " ")
		}
		// Workstream B: prepend the OS-sandbox prefix when this is a
		// source-writing phase AND the host can wrap. Non-Claude drivers
		// (codex/agy/ollama) get the same confinement Claude already gets via
		// PreToolUse hooks. When wrap is unavailable (nested-claude / no
		// sandbox binary / EVOLVE_SANDBOX=off), drivers run unwrapped —
		// trust kernel falls back to its Claude-only pre-B posture.
		if prefix, ok := sandboxPrefixForLaunch(deps, cfg); ok {
			launchCmd = joinPrefixForTmux(prefix) + " " + launchCmd
			fmt.Fprintf(deps.Stderr, "%s sandbox prefix applied (%d argv elements)\n", pfx, len(prefix))
		}
		_ = deps.Tmux.SendKeys(ctx, lp.session, launchCmd, true)
		fmt.Fprintf(deps.Stderr, "%s launching: %s\n", pfx, launchCmd)

		interval := lp.bootIntervalS
		if interval <= 0 {
			interval = 1
		}
		// ADR-0043 A0: accumulate the cold-boot wait. Seeded with the two fixed
		// deps.Sleep(time.Second) readiness waits above (post new-session, post
		// cd); each poll iteration adds its interval. Derived from the Sleep-driven
		// counter (not wall clock) so it is deterministic under the test no-op
		// Sleep and ≈ wall time in production. Keep fixedReadinessWaits in sync
		// with the count of fixed deps.Sleep(time.Second) calls above.
		const fixedReadinessWaits = 2
		bootWaitMS := int64(fixedReadinessWaits) * 1000
		promptSeen := false
		for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
			deps.Sleep(time.Duration(interval) * time.Second)
			bootWaitMS += int64(interval) * 1000
			pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			// Cycle-121 fix (codex-cli-0.134-repl-boot-timeout dossier, Fix B):
			// tick BEFORE the marker check. codex 0.134's trust modal renders
			// `›` (U+203A) as a "Yes, continue" selection bullet — the same
			// character codex-tmux uses as its REPL prompt marker. Pre-fix the
			// loop saw `›` in the pane, declared the REPL booted, and exited
			// before the auto-responder could press `1,Enter` to dismiss the
			// modal — which then hung the actual REPL launch behind it.
			// Running tick first lets the interactive_prompts regex match +
			// dismiss the modal so the true REPL prompt can appear cleanly.
			if lp.tickDuringBoot {
				ar.tick(ctx, lp.session) // codex/agy: handle trust prompts during boot
			}
			if strings.Contains(pane, lp.promptMarker) {
				if lp.bootMenuSkip != "" && tmuxPaneLooksLikeUpdateMenu(pane) {
					_ = deps.Tmux.SendKeys(ctx, lp.session, lp.bootMenuSkip, true)
					fmt.Fprintf(deps.Stderr, "%s boot interstitial dismissed before prompt delivery\n", pfx)
					continue
				}
				// Cycle-274 fix (codex-update-menu-swallows-injection): the
				// marker substring alone is not readiness — a stale marker in
				// scrollback above a DEAD SHELL (codex's updater exited to
				// zsh) read as ready and the injection landed in the shell.
				// Reject when the pane's foreground process is a known shell;
				// controllers without PaneCommander keep marker-only behavior.
				if lp.guardDeadShell {
					if shellCmd, isShell := paneShellProcess(ctx, deps.Tmux, lp.session); isShell {
						fmt.Fprintf(deps.Stderr, "%s marker visible but pane process is a shell (%s) — not ready (dead-shell guard)\n", pfx, shellCmd)
						continue
					}
				}
				promptSeen = true
				fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", pfx, lp.promptMarker)
				break
			}
		}
		if !promptSeen {
			fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", pfx, tmuxREPLBootTimeoutS)
			return ExitREPLBootTimeout, nil
		}
		// ADR-0043 A0: report cold-boot latency to the prompt marker. Only the
		// cold path reaches here; the warm/resumed named-session branch skips
		// this whole block, so OnBoot never fires there and BootMS stays 0.
		if deps.OnBoot != nil {
			deps.OnBoot(bootWaitMS)
		}
	}

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

	// --- Seed any launch-time REPL input (e.g. "/model sonnet") after the
	// boot marker, before the task prompt. Skipped on a resumed named
	// session (the seed already ran on the original launch).
	if !namedExists && len(cfg.Realization.REPLInput) > 0 {
		for _, ln := range cfg.Realization.REPLInput {
			_ = deps.Tmux.SendKeys(ctx, lp.session, ln, true)
			deps.Sleep(time.Second)
		}
		fmt.Fprintf(deps.Stderr, "%s seeded %d REPL input line(s)\n", pfx, len(cfg.Realization.REPLInput))
	}

	// --- Deliver the prompt via the paste buffer (human-input cadence when engaged).
	if human {
		humanBootPause(deps)
		humanPasteWithReview(ctx, deps, lp.session, resolvedPromptFile)
	} else {
		_ = deps.Tmux.LoadBuffer(ctx, lp.session, resolvedPromptFile)
		_ = deps.Tmux.PasteBuffer(ctx, lp.session)
		deps.Sleep(time.Second)
		_ = deps.Tmux.SendKeys(ctx, lp.session, "", true) // Enter
	}
	fmt.Fprintf(deps.Stderr, "%s prompt delivered\n", pfx)

	// --- Live-injection inbox cursor. Seek to EOF so a resumed named session
	// (or a stale ephemeral file) never replays a pre-launch backlog — only
	// envelopes appended AFTER the agent is running are delivered.
	cursor := inbox.NewCursor(cfg.Workspace, cfg.Agent)
	if fi, err := os.Stat(inbox.Path(cfg.Workspace, cfg.Agent)); err == nil {
		cursor.SetOffset(fi.Size())
	}

	// --- Live bidirectional channel (ADR-0037), opt-in via EVOLVE_CHANNEL=1.
	// When on, the driver is the SOLE writer of two live files the Producer
	// tails: <agent>-pane.live (newly-stabilized capture-pane content, extracted
	// per CLI by panestream.PaneDelta) and <agent>-breadcrumbs.live
	// (inject_applied / idle_reached correlation markers). Off → byte-identical:
	// both sinks are io.Discard, no files are created, and the per-tick delta
	// capture in the wait loop is skipped. A file that cannot be opened WARNs and
	// degrades to io.Discard — channel telemetry never aborts the phase.
	channelOn := channelEnabled(deps)
	// io.Discard has static type io.Writer (var Discard Writer = …), so := infers
	// io.Writer here and the *os.File reassignment below compiles.
	paneLiveW := io.Discard
	breadcrumbW := io.Discard
	var paneDelta panestream.PaneDelta
	// Resolve once: reused by the channel idle/busy bracket, the stop-review
	// busy-liveness signal, and the nudge gate below (all need the per-CLI
	// busy affordance). lp is immutable across the wait loop.
	paneProfile := paneProfileFor(lp)
	if channelOn {
		if f, err := os.OpenFile(filepath.Join(cfg.Workspace, cfg.Agent+"-pane.live"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			defer func() { _ = f.Close() }()
			paneLiveW = f
		} else {
			fmt.Fprintf(deps.Stderr, "%s WARN channel pane.live open: %v\n", pfx, err)
		}
		if f, err := os.OpenFile(filepath.Join(cfg.Workspace, cfg.Agent+"-breadcrumbs.live"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			defer func() { _ = f.Close() }()
			breadcrumbW = f
		} else {
			fmt.Fprintf(deps.Stderr, "%s WARN channel breadcrumbs.live open: %v\n", pfx, err)
		}
	}

	// --- Wait for the artifact in review intervals. A hard wall-clock deadline
	// kills a slow-but-productive agent (it cannot tell "stuck" from "still
	// thinking"). Instead, when a review interval elapses without the artifact,
	// a StopReviewer adjudicates the evidence — did the agent emit new output? —
	// into extend (still working, wait another interval) or pause (stalled,
	// surface for investigation, do not silently kill). See stopreview.go +
	// ADR-0026. The interval defaults to tmuxArtifactTimeoutS,
	// overridable per-launch (cfg.ArtifactTimeoutS) or via EVOLVE_ARTIFACT_TIMEOUT_S.
	interval := cfg.ArtifactTimeoutS
	if interval <= 0 {
		interval = envInt(deps, "EVOLVE_ARTIFACT_TIMEOUT_S", tmuxArtifactTimeoutS)
	}
	// Defensive default: the Engine path sets deps.Reviewer via withDefaults,
	// but direct runTmuxREPL callers (tests, future Stage-1 wiring) may not —
	// avoid a nil-deref at the review checkpoint.
	reviewer := deps.Reviewer
	if reviewer == nil {
		reviewer = newDeterministicReviewer(envInt(deps, "EVOLVE_ARTIFACT_MAX_EXTENDS", defaultArtifactMaxExtends))
	}
	// ADR-0044 C2: the fatal-pane registry consulted before each review
	// checkpoint (fatalpane.go). Stage off ⇒ fatalPaneVerdict short-circuits
	// before touching the detector; nil detector is unreachable on that path.
	recoveryStage := recoveryStageFromEnv(deps)
	var fatalDet *recovery.FatalPaneDetector
	if recoveryStage != "off" {
		// Seeds + durable advisor promotions (ADR-0044 Slice 5): a novel
		// signature classified once is caught deterministically on every
		// later boot. Empty ProjectRoot degrades to seeds only.
		fatalDet = recovery.SeedDetectorWithPromotions(
			filepath.Join(cfg.ProjectRoot, ".evolve", "instincts", "fatal-signatures"))
	}
	// ADR-0027: the completion contract is a Strategy. Default ("" / "artifact")
	// is the legacy artifact-file poll, byte-identical to the pre-Strategy code;
	// "stdout" completes on REPL-idle for agents that print their answer and
	// write no file (the router/advisor). The detector ONLY decides readiness —
	// the stop-review/extend liveness adjudication below is unchanged.
	var lastEv StopEvent
	var lastVerdict ReviewVerdict
	detector := newCompletionDetector(cfg.Completion, cfg, deps, lp)
	completed := false
	nudgeSent := false
	// ADR-0045 I1: the one-shot nudge's outcome window — resolved when the
	// run concludes, against the only evidence that matters: did the
	// artifact appear within the bounded wait? `nudgeSent=true` with no
	// outcome record is the cycles-263–269 defect this closes.
	var nudgeEv *interaction.Event
	var nudgeAt time.Time
	defer func() {
		if nudgeEv == nil {
			return
		}
		res := interaction.ResultNoEffect
		if completed {
			res = interaction.ResultArtifactAppeared
		}
		irec.Record(interaction.Outcome{
			Event:     *nudgeEv,
			Result:    res,
			LatencyMS: deps.Now().Sub(nudgeAt).Milliseconds(),
		})
	}()
	detectErrLogged := false
	peakTokens := 0
	recordTokens := func(pane string) {
		if n := extractTokenCount(pane); n > peakTokens {
			peakTokens = n
		}
	}
	attempt := 0
	intervalStart := 0
	// --- Correlation span tracking for the bidirectional channel (ADR-0037).
	// openCorrID is the CorrID of the most-recently-delivered idle-gated ask
	// that has not yet been answered. sawBusy guards against a false
	// idle_reached: the prompt marker is still visible immediately after the
	// paste (the agent hasn't started its turn yet), so we require observing
	// at least one BUSY pane (marker absent) before the next marker-visible
	// pane counts as the agent returning to idle. Heuristic given the 2s poll:
	// a turn shorter than one poll interval may be missed, in which case the
	// idle_reached fires on a later idle tick (the open CorrID persists until
	// a busy→idle pair is seen) — never a false-early bracket.
	openCorrID := ""
	sawBusy := false
	intervalBaselinePane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
	recordTokens(intervalBaselinePane)
	// Cycle-274 post-paste spill check (R3.2), on the ALREADY-captured
	// baseline (no extra capture, no fixture-frame drift): the prompt was
	// just pasted; if it spilled into a shell continuation (quote>/bquote>)
	// AND the pane's foreground process IS a shell (authoritative — a pane
	// merely quoting spill text under a live CLI must not trip this), the
	// CLI is gone. Fail fast as a transient so the fallback chain takes
	// over, instead of the 25-min wedge cycles 274/277 burned. Mid-run
	// process death past this boundary is the observer's job (plan R3.4).
	if lp.guardDeadShell && paneLooksLikeShellSpill(intervalBaselinePane) {
		if shellCmd, isShell := paneShellProcess(ctx, deps.Tmux, lp.session); isShell {
			fmt.Fprintf(deps.Stderr, "%s FAIL: prompt spilled into a dead shell (%s) after paste — CLI process gone (cycle-274 class)\n", pfx, shellCmd)
			return ExitREPLBootTimeout, nil
		}
	}
	for elapsed := 0; ; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		if err := ctx.Err(); err != nil {
			// Context cancelled (orchestrator timeout / SIGTERM): stop waiting
			// promptly rather than running out the reviewer's extend budget.
			// Load-bearing once a Stage-1 LLM reviewer can extend at length.
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning completion wait\n", pfx, err)
			break
		}
		// Live channel: stream newly-stabilized rendered content to pane.live.
		// The first Next() primes the baseline (echoed prompt + boot chrome are
		// counted, not emitted); later ticks emit only the assistant output that
		// appeared above the volatile input box. Gated so off adds no capture.
		if channelOn {
			if rendered, cerr := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback); cerr == nil {
				recordTokens(rendered)
				for _, ln := range paneDelta.Next(rendered, paneProfile) {
					fmt.Fprintln(paneLiveW, ln)
				}
			}
		}
		ready, _, note, derr := detector.poll(ctx)
		if ready {
			completed = true
			if note != "" {
				fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
			}
			break
		}
		if derr != nil && !detectErrLogged {
			// The detector surfaced a fault (e.g. an artifact present at a
			// non-canonical path that could not be relocated — read-only
			// workspace). Surface it once, immediately, instead of spinning the
			// full wait window with no signal.
			fmt.Fprintf(deps.Stderr, "%s WARN: completion detector: %v\n", pfx, derr)
			detectErrLogged = true
		}
		// Drain live-injection envelopes BEFORE the auto-respond tick so an
		// operator interrupt pre-empts a pending auto-reply on this tick.
		if envs, _ := cursor.Drain(); len(envs) > 0 {
			for _, env := range envs {
				// injectEnvelope returns a non-empty CorrID only when an
				// idle-gated correlated ask was actually pasted (not re-queued,
				// dropped, or a keystroke/interrupt). The breadcrumb is emitted
				// HERE — at the moment delivery is confirmed — so the channel sink
				// (and the open-span tracking) lives entirely in this loop. Gated:
				// channel off → no breadcrumb, no span tracking.
				cid := injectEnvelope(ctx, cfg, deps, lp, env)
				if cid != "" && channelOn {
					// A correlated ask was just delivered: open its span. A new
					// ask supersedes any prior unanswered one (its idle_reached
					// is then unobservable; the producer ignores an idle_reached
					// with no matching open inject, so dropping it is safe).
					emitChannelBreadcrumb(breadcrumbW, "inject_applied", cid)
					openCorrID = cid
					sawBusy = false
				}
			}
		}
		// Bracket the open ask: once delivered, watch for the agent going BUSY
		// (marker gone) and then back to IDLE (marker visible again), and emit
		// idle_reached exactly once on that busy→idle transition.
		if channelOn && openCorrID != "" {
			pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			// Busy/idle is NOT the prompt-marker's presence — the input box
			// persists during generation for claude/agy (and ollama echoes the
			// marker on the prompt line). panestream.PaneBusy reads the real
			// per-CLI busy signal (interrupt/spinner affordance, or ollama's
			// vanished idle placeholder). idle_reached fires once on busy→idle.
			if panestream.PaneBusy(pane, paneProfile) {
				sawBusy = true
			} else if sawBusy {
				emitChannelBreadcrumb(breadcrumbW, "idle_reached", openCorrID)
				openCorrID = ""
				sawBusy = false
			}
		}
		action, rc := ar.tick(ctx, lp.session)
		switch rc {
		case 0, 1: // noop / responded
		case 2:
			// Agent self-signalled progress ("extend_timeout"): restart the
			// current review interval so the signal counts as activity. Bounded
			// by the auto-respond loop guard (case 86) — an agent cannot defer
			// the reviewer indefinitely by repeating the same extend prompt.
			if parseExtendSecs(action) > 0 {
				intervalStart = elapsed
				fmt.Fprintf(deps.Stderr, "%s agent extend signal — review interval refreshed\n", pfx)
			}
		case 85:
			fmt.Fprintf(deps.Stderr, "%s auto-respond escalation; abandoning run\n", pfx)
			return ExitUnknownPrompt, nil
		case 86:
			fmt.Fprintf(deps.Stderr, "%s auto-respond loop guard tripped; abandoning run\n", pfx)
			return ExitRespondLoopGuard, nil
		}
		// Review checkpoint: a full interval elapsed without the artifact.
		if elapsed-intervalStart >= interval {
			rawPane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			curPane, renderWedged := recoverBlankPane(ctx, deps, lp.session, lp.bootScrollback, rawPane, pfx)
			recordTokens(curPane)
			// Progressed = the pane changed during the interval. Stage-0 signal:
			// good for the common cases (growing token counters, new tool calls),
			// but a pure spinner/clock animation also reads as progress — so the
			// maxExtends backstop, not this diff, bounds a spinner-stuck agent
			// (~maxExtends×interval). Stage 1's reviewer inspects StdoutTail to
			// disambiguate genuine work from animation.
			progressed := PaneHasSubstantiveChange(intervalBaselinePane, curPane)
			lastEv = StopEvent{
				Kind:       StopArtifactTimeout,
				Phase:      cfg.Agent,
				Cycle:      cfg.Cycle,
				ElapsedS:   elapsed,
				IntervalS:  interval,
				Attempt:    attempt,
				Progressed: progressed,
				Busy:       panestream.PaneBusy(curPane, paneProfile) || renderWedged,
				StdoutTail: lastLines(curPane, 40),
			}
			// ADR-0044 C2: a known-fatal pane (model-invalid boot, CLI
			// self-update, dead shell) preempts the reviewer in enforce —
			// cycle-262's dead panes read as "progressed" because the
			// bridge's own nudge echoed into them, so the legacy
			// extend-while-progressing flow burned the full maxExtends
			// backstop on REPLs that no longer existed.
			// fatalPaneVerdict RECORDS a C2 evidence outcome on every
			// matching call (R8.3) — it must be called exactly once per
			// stop-review checkpoint, never retried for the same event, or
			// the soak's C2 counts inflate silently.
			v, preempted := fatalPaneVerdict(fatalDet, lastEv, recoveryStage, irec, deps.Stderr, pfx)
			if !preempted {
				v = reviewer.Review(lastEv)
			}
			lastVerdict = v
			fmt.Fprintf(deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
				pfx, StopArtifactTimeout, elapsed, attempt, progressed, lastVerdict.Action, lastVerdict.Reason)
			if deps.OnStopReview != nil {
				deps.OnStopReview(phaseName, string(lastVerdict.Action), lastVerdict.Reason)
			}
			if lastVerdict.Action != ReviewExtend {
				_, isDetVal := reviewer.(deterministicReviewer)
				_, isDetPtr := reviewer.(*deterministicReviewer)
				isDeterministic := isDetVal || isDetPtr
				// Nudge only on PAUSE (idle agent, remind it once). A fatal
				// ReviewStop (ADR-0044 C2) must exit now — nudging a dead
				// shell is exactly the echo that bought cycle-262's dead
				// panes their extensions. Behavior-identical for the legacy
				// reviewer, which only ever emits extend|pause.
				if lastVerdict.Action == ReviewPause && isDeterministic && !panestream.PaneBusy(curPane, paneProfile) && !nudgeSent {
					nudgeMsg := fmt.Sprintf("Please write the deliverable to %s to complete the phase.", cfg.Artifact)
					_ = deps.Tmux.SendKeys(ctx, lp.session, nudgeMsg, true)
					fmt.Fprintf(deps.Stderr, "%s idle with missing artifact; sent one-shot nudge: %s\n", pfx, nudgeMsg)
					nudgeSent = true
					nudgeEv = &interaction.Event{
						Kind:    interaction.KindNudge,
						Phase:   phaseName,
						Cycle:   cfg.Cycle,
						Trigger: "idle_no_artifact",
						Payload: nudgeMsg,
					}
					nudgeAt = deps.Now()
					intervalStart = elapsed
					intervalBaselinePane = curPane
					attempt++
					continue
				}
				break
			}
			attempt++
			intervalStart = elapsed
			intervalBaselinePane = curPane
		}
	}
	if !completed {
		fmt.Fprintf(deps.Stderr, "%s FAIL: completion never signalled (artifact %s; stop-review paused after %d interval(s) of %ds)\n", pfx, cfg.Artifact, attempt+1, interval)
		fmt.Fprintf(deps.Stderr, "%s diagnostic: files present under workspace %s:\n", pfx, cfg.Workspace)
		for _, line := range listWorkspaceFiles(cfg.Workspace) {
			fmt.Fprintf(deps.Stderr, "%s   %s\n", pfx, line)
		}
		// Pause (ambiguous stall) and Stop (typed fatal fast-fail, ADR-0044
		// C2) both leave the operator-facing escalation report; extend keeps
		// the legacy no-report behavior.
		if lastVerdict.Action == ReviewPause || lastVerdict.Action == ReviewStop {
			_ = writeEscalationReport(cfg.Workspace, phaseName, cfg.Cycle, lastEv, lastVerdict)
		}
		return ExitArtifactTimeout, nil
	}

	// --- Capture scrollback: raw → stderr-log, ANSI-stripped → stdout-log.
	raw, _ := deps.Tmux.CapturePane(ctx, lp.session, artifactScrollback)
	recordTokens(raw)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
	writeTokenUsage(cfg.Workspace, peakTokens)
	fmt.Fprintf(deps.Stderr, "%s scrollback captured\n", pfx)

	if lp.named {
		fmt.Fprintf(deps.Stderr, "%s RESUME-PRESERVE: skipping exit; REPL stays running for next launch\n", pfx)
	} else {
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

func tmuxPaneLooksLikeUpdateMenu(pane string) bool {
	return strings.Contains(pane, "Update available!") &&
		strings.Contains(pane, "Update now") &&
		strings.Contains(pane, "Skip")
}

// isShellProcess reports whether a pane_current_command value names a known
// interactive shell. The set is closed and reject-listed (vs requiring a
// known CLI binary) because CLI process names vary by runtime — claude runs
// under "node", codex is "codex" — while a wedged pane is always one of
// these. Login shells report with a leading dash ("-zsh").
func isShellProcess(cmd string) bool {
	switch strings.TrimPrefix(cmd, "-") {
	case "zsh", "bash", "sh", "fish", "dash", "tcsh", "ksh":
		return true
	}
	return false
}

// paneShellProcess asks the controller (when it implements the optional
// PaneCommander capability) for the pane's foreground process and reports
// whether it is a shell. (cmd, false) when the capability is absent, the
// query fails, or the process is not a shell — all degrade to the
// pre-handshake marker-only behavior.
func paneShellProcess(ctx context.Context, tm TmuxController, session string) (string, bool) {
	pc, ok := tm.(PaneCommander)
	if !ok {
		return "", false
	}
	cmd, err := pc.PaneCommand(ctx, session)
	if err != nil {
		return "", false
	}
	return cmd, isShellProcess(cmd)
}

// paneLooksLikeShellSpill reports the cycle-274 paste-spill signatures: a
// shell continuation prompt (quote>/bquote>/dquote>/heredoc>) as the LAST
// non-blank line (continuation prompts only ever render at the cursor), or
// zsh's command-not-found echo anywhere. Callers MUST pair this with the
// authoritative paneShellProcess check — agent output may legitimately quote
// these strings.
func paneLooksLikeShellSpill(pane string) bool {
	if strings.Contains(pane, "command not found") {
		return true
	}
	lines := strings.Split(strings.TrimRight(pane, "\n \t"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	for _, w := range []string{"quote>", "bquote>", "dquote>", "heredoc>"} {
		if last == w {
			return true
		}
	}
	return false
}

// injectEnvelope delivers one live-injection envelope into the running REPL.
// command/nudge/system_rule are idle-gated (injected only when the prompt
// marker is visible); a mid-turn arrival is re-queued, bounded by
// maxInjectDefer. interrupt sends ESC first, then injects regardless of state.
// keystroke sends body as raw tmux key tokens — no ESC prefix, no idle-gate,
// no Enter suffix; the operator owns exactly what reaches the REPL.
//
// It returns the CorrID of a successfully-delivered idle-gated ask (empty
// otherwise: non-correlated body, keystroke/interrupt path, re-queue, or drop).
// Only this function knows the idle-gate passed (vs re-queued/dropped), so the
// non-empty return is the caller's signal in runTmuxREPL to emit inject_applied
// and open the busy→idle span for idle_reached.
func injectEnvelope(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch, env inbox.Envelope) string {
	pfx := "[" + lp.name + "]"
	// Cycle-124 F4 / ADR-0023 addendum: the "full tmux control" hatch the
	// operator asked for. Body is one tmux key-spec (literal text and/or
	// space-separated named keys like "Enter" / "Escape" / "C-c" / "Up" /
	// "y Enter") sent verbatim via SendKeys with enter=false. NO idle-gate
	// (operator may need to send keys precisely BECAUSE the agent isn't
	// idle — e.g. dismissing a modal that hung mid-turn), NO ESC prefix
	// (unlike interrupt), NO automatic Enter append. Empty body is a no-op
	// to match the existing SendKeys contract (line 59 of tmux.go skips
	// empty key strings). The operator is fully responsible for what they
	// inject; the bridge does not interpret the body.
	if env.Kind == inbox.KindKeystroke {
		// Warn-not-block: flag tokens that look like a mistyped key name
		// (e.g. "Excape") so an operator notices the keystroke will be typed
		// verbatim rather than acted on — but NEVER refuse the send (this is
		// the full-control hatch; literal text is a legitimate body).
		if suspect := keyspec.Validate(env.Body); len(suspect) > 0 {
			fmt.Fprintf(deps.Stderr, "%s keystroke WARN: unrecognized key token(s) %v in %q — sending verbatim\n", pfx, suspect, env.Body)
		}
		// Surface a failed send instead of logging success unconditionally
		// (cycle-124 review MEDIUM): a vanished session / killed pane would
		// otherwise show as `injected keystroke "Enter"` on stderr while
		// nothing actually reached the REPL.
		if err := deps.Tmux.SendKeys(ctx, lp.session, env.Body, false); err != nil {
			fmt.Fprintf(deps.Stderr, "%s keystroke send failed: %v (source=%s)\n", pfx, err, env.Source)
			return ""
		}
		fmt.Fprintf(deps.Stderr, "%s injected keystroke %q (source=%s)\n", pfx, env.Body, env.Source)
		return ""
	}
	if env.Kind == inbox.KindInterrupt {
		_ = deps.Tmux.SendKeys(ctx, lp.session, "Escape", false)
		deps.Sleep(injectInterruptSettle)
		_ = injectText(ctx, cfg, deps, lp.session, env.Body) // fire-and-forget live injection
		fmt.Fprintf(deps.Stderr, "%s injected interrupt (source=%s)\n", pfx, env.Source)
		return ""
	}

	// Idle-gated kinds: only inject when the agent is waiting at the prompt.
	pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
	if !strings.Contains(pane, lp.promptMarker) {
		if env.DeferCount >= maxInjectDefer {
			fmt.Fprintf(deps.Stderr, "%s DROP injected %s after %d defers (agent never idled)\n", pfx, env.Kind, env.DeferCount)
			return ""
		}
		env.DeferCount++
		if err := inbox.Append(cfg.Workspace, cfg.Agent, env, deps.Now); err != nil {
			fmt.Fprintf(deps.Stderr, "%s WARN re-queue of %s failed: %v\n", pfx, env.Kind, err)
		}
		return ""
	}

	body := env.Body
	if env.Kind == inbox.KindSystemRule {
		body = "## Rules\n" + body
	}
	_ = injectText(ctx, cfg, deps, lp.session, body) // fire-and-forget live injection
	fmt.Fprintf(deps.Stderr, "%s injected %s (source=%s)\n", pfx, env.Kind, env.Source)
	// Return the CorrID of the just-delivered idle-gated ask so runTmuxREPL can
	// emit the inject_applied breadcrumb (to the channel sink it owns) and open
	// the busy→idle span. Empty CorrID = uncorrelated inject → caller no-ops.
	return env.CorrID
}

// injectText delivers body into the session via the paste buffer (so
// multi-line/special characters survive — SendKeys would mangle them), then
// Enter. It uses a dedicated scratch file so it never collides with the task
// prompt's resolved-prompt.txt. Returns the first transport error so callers
// that gate on delivery (the recipe engine) can surface a dead session
// instead of waiting out a full timeout; the fire-and-forget live-injection
// callers ignore it (preserving prior behavior).
func injectText(ctx context.Context, cfg *Config, deps Deps, session, body string) error {
	scratch := filepath.Join(cfg.Workspace, ".bridge-inbox", orDefault(cfg.Agent, "agent")+"-inject.txt")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch mkdir: %v\n", session, err)
		return fmt.Errorf("inject scratch mkdir: %w", err)
	}
	if err := os.WriteFile(scratch, []byte(body), 0o644); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch write: %v\n", session, err)
		return fmt.Errorf("inject scratch write: %w", err)
	}
	if err := deps.Tmux.LoadBuffer(ctx, session, scratch); err != nil {
		return fmt.Errorf("inject load-buffer: %w", err)
	}
	if err := deps.Tmux.PasteBuffer(ctx, session); err != nil {
		return fmt.Errorf("inject paste-buffer: %w", err)
	}
	deps.Sleep(time.Second)
	return deps.Tmux.SendKeys(ctx, session, "", true) // Enter
}

// resolveSession returns the tmux session name and whether it is a stable
// named (resume-eligible) session. ephemeralPrefix distinguishes drivers
// (evolve-bridge- / evolve-bridge-codex- / evolve-bridge-agy-). Named
// sessions (claude-tmux only) use evolve-bridge-named-<name>.
func resolveSession(cfg *Config, deps Deps, ephemeralPrefix string) (session string, named bool) {
	if cfg.SessionName != "" {
		return NamedSessionName(cfg.SessionName), true
	}
	agent := orDefault(cfg.Agent, "probe")
	s := fmt.Sprintf("%sc%d-%s-pid%d-%d", ephemeralPrefix, cfg.Cycle, agent, os.Getpid(), deps.Now().Unix())
	return truncate64(s), false
}

func truncate64(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

// NamedSessionName returns the tmux session name for a swarm-controlled named
// session. It is the single source of truth shared by resolveSession (which
// creates the session) and the swarm reaper (which kills it by this name).
// Format: "evolve-bridge-named-<name>", truncated to 64 characters.
func NamedSessionName(name string) string {
	return truncate64("evolve-bridge-named-" + name)
}

// parseExtendSecs parses an "extend:<secs>" auto-respond action.
func parseExtendSecs(action string) int {
	const p = "extend:"
	if !strings.HasPrefix(action, p) {
		return 0
	}
	n := 0
	for _, c := range action[len(p):] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// recoverBlankPane handles the claude ≥2.1.173 BLANK-PANE render wedge
// (inbox claude-2.1.173-blank-pane-after-interval): an EMPTY capture while
// the session is alive is the Ink renderer wedging, not idleness —
// cycle-291's agent kept working behind a blank pane and the stall-pause
// burned interval×attempts to exit=81. Recovery: jiggle the window width
// (two SIGWINCHes → full repaint, windowJiggler optional capability) and
// re-read. Returns the freshest pane and whether the wedge persisted — a
// still-blank pane must read BUSY (extend; never pause a live agent on a
// pane that stopped rendering; the maxExtends backstop still bounds it).
func recoverBlankPane(ctx context.Context, deps Deps, session string, scrollback int, pane, pfx string) (string, bool) {
	if strings.TrimSpace(pane) != "" || !deps.Tmux.HasSession(ctx, session) {
		return pane, false
	}
	if j, ok := deps.Tmux.(windowJiggler); ok {
		_ = j.JiggleWindow(ctx, session)
		deps.Sleep(time.Second)
	}
	if re, err := deps.Tmux.CapturePane(ctx, session, scrollback); err == nil && strings.TrimSpace(re) != "" {
		fmt.Fprintf(deps.Stderr, "%s render wedge: blank pane redrawn after jiggle\n", pfx)
		return re, false
	}
	fmt.Fprintf(deps.Stderr, "%s render wedge: pane still blank after jiggle — treating live session as busy\n", pfx)
	return pane, true
}

// tmuxCleanup captures final scrollback then kills the session — unless it
// is a named session, which is preserved for resume.
func tmuxCleanup(ctx context.Context, deps Deps, name, session, scrollbackFile string, named bool, scrollback int) {
	pfx := "[" + name + "]"
	if !deps.Tmux.HasSession(ctx, session) {
		return
	}
	if raw, err := deps.Tmux.CapturePane(ctx, session, scrollback); err == nil {
		_ = os.WriteFile(scrollbackFile, []byte(raw), 0o644)
	}
	if named {
		fmt.Fprintf(deps.Stderr, "%s session PRESERVED for resume: %s\n", pfx, session)
		return
	}
	_ = deps.Tmux.KillSession(ctx, session)
	fmt.Fprintf(deps.Stderr, "%s session killed: %s\n", pfx, session)
}

func writeTokenUsage(workspace string, peakTokens int) {
	if peakTokens < 0 {
		peakTokens = 0
	}
	data, err := json.MarshalIndent(struct {
		PeakTokens int `json:"peak_tokens"`
	}{PeakTokens: peakTokens}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(workspace, "token-usage.json"), append(data, '\n'), 0o644)
}

// tmuxNonClaudePreflight runs the rejections shared by codex-tmux and
// agy-tmux: permission_mode is claude-only, named sessions are
// claude-tmux-only, and --allow-bypass is mandatory (these drivers run
// the inner CLI with bypass-like semantics). Returns (exitCode, handled);
// when handled, the driver returns immediately.
func tmuxNonClaudePreflight(name string, cfg *Config, deps Deps) (int, bool) {
	pfx := "[" + name + "]"
	if cfg.PermissionMode != "" {
		fmt.Fprintf(deps.Stderr, "%s permission_mode='%s' is not supported on this CLI\n", pfx, cfg.PermissionMode)
		fmt.Fprintf(deps.Stderr, "%s Only claude-p and claude-tmux drivers support --permission-mode.\n", pfx)
		return ExitBadFlags, true
	}
	if cfg.StreamOutput {
		fmt.Fprintf(deps.Stderr, "%s NOTE: stream_output=true is not supported on this CLI — no-op\n", pfx)
	}
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "%s --session-name='%s' is not supported on this CLI in v0.5\n", pfx, cfg.SessionName)
		fmt.Fprintf(deps.Stderr, "%s Only claude-tmux supports named/resumable sessions; use --cli=claude-tmux or omit --session-name.\n", pfx)
		return ExitBadFlags, true
	}
	if !cfg.AllowBypass {
		fmt.Fprintf(deps.Stderr, "%s safety gate: --allow-bypass is required\n", pfx)
		return ExitSafetyGate, true
	}
	return 0, false
}

type escalationReport struct {
	Phase     string `json:"phase"`
	Cycle     int    `json:"cycle"`
	ElapsedS  int    `json:"elapsed_s"`
	IntervalS int    `json:"interval_s"`
	Attempt   int    `json:"attempt"`
	StopKind  string `json:"stop_kind"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	FinalPane string `json:"final_pane"`
}

func writeEscalationReport(workspace, phase string, cycle int, ev StopEvent, verdict ReviewVerdict) error {
	report := escalationReport{
		Phase:     phase,
		Cycle:     cycle,
		ElapsedS:  ev.ElapsedS,
		IntervalS: ev.IntervalS,
		Attempt:   ev.Attempt,
		StopKind:  string(ev.Kind),
		Action:    string(verdict.Action),
		Reason:    verdict.Reason,
		FinalPane: ev.StdoutTail,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspace, fmt.Sprintf("%s-escalation-report.json", phase))
	return os.WriteFile(path, data, 0o644)
}
