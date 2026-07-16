package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliadmit"
	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
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
	pfx := "[" + lp.name + "]"

	workingDir := cfg.Worktree
	if workingDir == "" {
		// CB.2: the cwd fallback launches the agent over WHATEVER directory
		// the dispatching process sits in. Under a fleet supervisor that may
		// be another run's tree — fail closed (typed, exit 10 is a non-trigger
		// code: a config bug must surface, never CLI-fallback). Single mode
		// keeps the fallback for operator ergonomics, but loudly.
		if v, _ := lookupEnv(deps, ipcenv.FleetKey); envchain.BoolValue(v, false) {
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
	var resolvedPromptFile, resolvedPrompt string
	if !lp.bootOnly {
		prompt, err := preparePrompt(cfg, deps)
		if err != nil {
			return ExitBadFlags, err
		}
		resolvedPrompt = prompt
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
	artifactScrollback := defaultIfZero(deps.ScrollbackLines, tmuxArtifactScrollback)
	fmt.Fprintf(deps.Stderr, "%s session=%s model=%s workdir=%s\n", pfx, lp.session, cfg.Model, workingDir)
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

	// --- Spawn + cd + launch + wait for the REPL prompt marker.
	if !namedExists {
		// Slice-4: cross-process CLI admission control. max<=0 (the default) is
		// unbounded — byte-identical to the pre-Slice-4 path. On error, degrade
		// gracefully: proceed uncapped + WARN (admission control must never block
		// a phase outright). EVOLVE_CLI_MAX_CONCURRENT_<CLI> opts in.
		admitMax := envInt(deps, "EVOLVE_CLI_MAX_CONCURRENT_"+strings.ToUpper(lp.name), 0)
		admitRelease, admitErr := cliadmit.Acquire(ctx, lp.name, admitMax, cliadmit.DefaultTTL)
		if admitErr != nil {
			fmt.Fprintf(deps.Stderr, "%s WARN cliadmit: %v (proceeding uncapped)\n", pfx, admitErr)
		}
		defer admitRelease()

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
		// CB.5: record the session in the run's registry the moment it exists,
		// so teardown reaps exactly what this run created (by registry, never
		// glob) even if this launch crashes before its own deferred cleanup.
		// Best-effort: a failed write degrades to today's leak-on-crash, loudly.
		if err := sessionrecord.Append(sessionrecord.PathIn(cfg.Workspace), sessionrecord.Record{
			Session: lp.session, RunID: cfg.RunID, Cycle: cfg.Cycle,
			Agent: cfg.Agent, PID: os.Getpid(), CreatedAt: deps.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			fmt.Fprintf(deps.Stderr, "%s WARN session registry append failed: %v (session %s will not be registry-reapable)\n", pfx, err, lp.session)
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
		// Boot-wait deadline (seconds). Configurable via BridgePolicy.BootTimeoutS
		// (policy.json bridge.boot_timeout_s) so a loaded CI runner can raise the
		// fake-CLI/tmux handshake budget — the fixed 60s intermittently flaked the
		// integration tier ("REPL prompt never appeared after 60s").
		bootDeadlineS := defaultIfZero(deps.BootTimeoutS, tmuxREPLBootTimeoutS)
		// ADR-0043 A0: accumulate the cold-boot wait. Seeded with the two fixed
		// deps.Sleep(time.Second) readiness waits above (post new-session, post
		// cd); each poll iteration adds its interval. Derived from the Sleep-driven
		// counter (not wall clock) so it is deterministic under the test no-op
		// Sleep and ≈ wall time in production. Keep fixedReadinessWaits in sync
		// with the count of fixed deps.Sleep(time.Second) calls above.
		const fixedReadinessWaits = 2
		bootWaitMS := int64(fixedReadinessWaits) * 1000
		promptSeen := false
		for elapsed := 0; elapsed < bootDeadlineS; elapsed += interval {
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
				// codex/agy/claude: dismiss boot dialogs (trust prompts) BEFORE the
				// marker check (Cycle-121). A fire-once dialog's selection cursor can
				// BE the REPL marker (claude v2.1.193's ❯ trust dialog, codex's ›
				// modal), so the pane captured ABOVE still shows that dialog after the
				// tick dismisses it. Declaring ready on that stale frame and pasting
				// the prompt delivers it INTO the dialog, where it is lost (claude
				// v2.1.193 rc=81). When a once-dialog was just dismissed, re-poll so
				// the next iteration sees the real REPL. Gated on a once-dialog
				// dismiss (not any auto-respond) so a repeating in-REPL prompt still
				// falls through to the wait-loop guard instead of spinning boot.
				ar.tick(ctx, lp.session)
				if ar.firedOnceThisTick {
					continue
				}
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
			fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", pfx, bootDeadlineS)
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

	// --- Live bidirectional channel (ADR-0037), implied by EVOLVE_PHASE_RECOVERY=enforce.
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
	// overridable per-launch (cfg.ArtifactTimeoutS) or via Deps.ArtifactTimeoutS (BridgePolicy).
	interval := cfg.ArtifactTimeoutS
	if interval <= 0 {
		interval = defaultIfZero(deps.ArtifactTimeoutS, tmuxArtifactTimeoutS)
	}
	// Defensive default: the Engine path sets deps.Reviewer via withDefaults,
	// but direct runTmuxREPL callers (tests, future Stage-1 wiring) may not —
	// avoid a nil-deref at the review checkpoint.
	reviewer := deps.Reviewer
	if reviewer == nil {
		reviewer = newDeterministicReviewer(defaultIfZero(deps.ArtifactMaxExtends, defaultArtifactMaxExtends))
	}
	// SignalCenter (ADR-0068, S3): the authoritative liveness source for the
	// checkpoint below — Observe+Aggregate replace the bare per-run detectorFor(lp)
	// probe. deps.LivenessCenter injects a shared/test instance; nil (production)
	// builds a private center whose empty registry makes Observe fall back to
	// panestream.DetectorFor(profile) — the SAME probe detectorFor(lp) built, so
	// the migration is verdict-identical (H1) unless a caller has registered a
	// handler for this profile's name.
	livenessCenter := deps.LivenessCenter
	if livenessCenter == nil {
		livenessCenter = panestream.NewSignalCenter()
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
		if n := panestream.ExtractResponseTokens(pane); n > peakTokens {
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
	// CB.6: the freshest non-empty pane seen — escalation evidence that
	// survives a mid-phase server death (cycle-286 masked-evidence class).
	lastGoodPane := intervalBaselinePane
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
			// Context cancelled (orchestrator timeout / SIGTERM / the next phase
			// tearing down this session): before abandoning, ONE final completion
			// poll — a deliverable already on disk means the session COMPLETED and
			// the cancel is benign teardown, not a phase timeout. Pre-fix this
			// break skipped straight to the !completed → ExitArtifactTimeout exit,
			// laundering a finished session into a timeout (session-lifecycle
			// residual; the runner's settle-retry was the only thing standing
			// between that mislabel and a false FAIL). The artifact detector is a
			// pure file stat, so the dead ctx cannot fail this last look; a
			// genuinely unfinished session still exits ExitArtifactTimeout.
			if ready, _, note, _ := detector.poll(ctx); ready {
				completed = true
				if note != "" {
					fmt.Fprintf(deps.Stderr, "%s %s\n", pfx, note)
				}
				fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) AFTER completion — benign teardown of a finished session\n", pfx, err)
				break
			}
			// Load-bearing once a Stage-1 LLM reviewer can extend at length:
			// stop waiting promptly rather than running out the extend budget.
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
			// marker on the prompt line). livenessCenter.BusyOf reads the real
			// per-CLI busy signal (interrupt/spinner affordance, or ollama's
			// vanished idle placeholder) via the same stateless projection
			// PaneBusy defines — routed through the center (cycle-434 S4
			// completion) rather than called directly, so no bridge consumer
			// parses CLI chrome outside SignalCenter. idle_reached fires once
			// on busy→idle.
			if livenessCenter.BusyOf(pane, paneProfile) {
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
			// CB.6: evidence survives the session's death. When the server
			// is killed mid-phase every later capture is empty, so the
			// escalation report's final_pane carried nothing and cycle-286's
			// retro misattributed the failure. Retain the last NON-EMPTY
			// pane; a dead capture falls back to it as the freshest real
			// evidence (the live pane still wins whenever it renders).
			if strings.TrimSpace(curPane) != "" {
				lastGoodPane = curPane
			}
			evidencePane := lastGoodPane
			// Progressed = the pane changed during the interval. Stage-0 signal:
			// good for the common cases (growing token counters, new tool calls),
			// but a pure spinner/clock animation also reads as progress — so the
			// maxExtends backstop, not this diff, bounds a spinner-stuck agent
			// (~maxExtends×interval). Stage 1's reviewer inspects StdoutTail to
			// disambiguate genuine work from animation.
			livenessCenter.Observe(lp.session, curPane, paneProfile)
			livenessState := livenessCenter.Aggregate()
			// Exhaustion fast-fail (SignalCenter ExhaustionProbe, ADR-0068): the
			// pane shows a quota/rate-limit WALL, so the artifact will NEVER come —
			// fail over to the fallback CLI (exit 85) NOW instead of nudging and
			// burning the full artifact timeout. Without this the re-printed error
			// reads as Converging and wedges the phase in an extend-forever livelock
			// (the agy hang-without-exit incident).
			if livenessState == panestream.LivenessExhausted {
				fmt.Fprintf(deps.Stderr, "%s EXHAUSTED: pane shows a quota/rate-limit wall — failing over to fallback CLI (exit %d)\n", pfx, ExitUnknownPrompt)
				return ExitUnknownPrompt, nil
			}
			// Progressed is sourced from the center's Changed(session)
			// projection (S4) — a consecutive-observation comparison, not the
			// interval-baseline diff. Behavior-preserving: .Progressed has no
			// decision consumer (evidence/logging only), so the shift from
			// baseline-relative to checkpoint-to-checkpoint is safe.
			progressed := livenessCenter.Changed(lp.session)
			// Render-wedge override (cycle-291): a blank pane from a live session
			// reads as Idle by the content-velocity detector (no affordance in blank
			// frame). recoverBlankPane already confirmed the session is alive
			// (renderWedged=true); treat as BusyButStagnant so the reviewer extends
			// rather than pausing a working agent on a pane-rendering failure.
			if renderWedged && livenessState == panestream.LivenessIdle {
				livenessState = panestream.LivenessBusyButStagnant
			}
			lastEv = StopEvent{
				Kind:       StopArtifactTimeout,
				Phase:      cfg.Agent,
				Cycle:      cfg.Cycle,
				ElapsedS:   elapsed,
				IntervalS:  interval,
				Attempt:    attempt,
				Progressed: progressed,
				Busy:       livenessCenter.Busy(lp.session) || renderWedged,
				StdoutTail: lastLines(evidencePane, 40),
				State:      livenessState,
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
				if lastVerdict.Action == ReviewPause && isDeterministic && !livenessCenter.Busy(lp.session) && !nudgeSent {
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
					attempt++
					continue
				}
				break
			}
			attempt++
			intervalStart = elapsed
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
	raw, capErr := deps.Tmux.CapturePane(ctx, lp.session, artifactScrollback)
	if raw == "" && capErr != nil {
		// Benign-cancel completion (the on-cancel final poll above): the dead
		// ctx cannot fork tmux, so this capture fails — fall back to the
		// freshest pane the wait loop observed (CB.6 lastGoodPane) instead of
		// writing empty logs and losing the forensic record for exactly the
		// teardown this path exists to classify correctly.
		raw = lastGoodPane
	}
	recordTokens(raw)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
	writeTokenUsage(cfg.Workspace, peakTokens)
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
