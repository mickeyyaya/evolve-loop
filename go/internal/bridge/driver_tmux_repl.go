package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
)

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
	exitSeq        []tmuxKey // keystrokes to close the REPL cleanly
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
		workingDir, _ = os.Getwd()
	}
	if !isDir(workingDir) {
		fmt.Fprintf(deps.Stderr, "%s working dir does not exist: %s\n", pfx, workingDir)
		return ExitBadFlags, nil
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}
	if err := ensureDirs(cfg); err != nil {
		return ExitBadFlags, fmt.Errorf("%s %w", pfx, err)
	}
	resolvedPromptFile := filepath.Join(cfg.Workspace, "resolved-prompt.txt")
	if err := os.WriteFile(resolvedPromptFile, []byte(prompt+"\n"), 0o644); err != nil {
		return ExitBadFlags, fmt.Errorf("%s write resolved prompt: %w", pfx, err)
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
	fmt.Fprintf(deps.Stderr, "%s session=%s model=%s workdir=%s\n", pfx, lp.session, cfg.Model, workingDir)
	defer tmuxCleanup(ctx, deps, lp.name, lp.session, scrollbackFile, lp.named)

	// Auto-respond fallback engine, seeded from the CLI's manifest rules.
	human := humanActive(deps, cfg.HumanInput)
	ar := newAutoResponder(lp.name, cfg.Workspace, deps, human)

	// --- Spawn + cd + launch + wait for the REPL prompt marker.
	if !namedExists {
		if err := deps.Tmux.NewSession(ctx, lp.session, tmuxPaneWidth, tmuxPaneHeight); err != nil {
			return ExitBadFlags, fmt.Errorf("%s new-session: %w", pfx, err)
		}
		deps.Sleep(time.Second)
		_ = deps.Tmux.SendKeys(ctx, lp.session, "cd "+workingDir, true)
		deps.Sleep(time.Second)
		launchCmd := lp.launchCmd
		if len(cfg.ExtraFlags) > 0 {
			launchCmd += " " + strings.Join(cfg.ExtraFlags, " ")
		}
		_ = deps.Tmux.SendKeys(ctx, lp.session, launchCmd, true)
		fmt.Fprintf(deps.Stderr, "%s launching: %s\n", pfx, launchCmd)

		interval := lp.bootIntervalS
		if interval <= 0 {
			interval = 1
		}
		promptSeen := false
		for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed += interval {
			deps.Sleep(time.Duration(interval) * time.Second)
			pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
			if strings.Contains(pane, lp.promptMarker) {
				promptSeen = true
				fmt.Fprintf(deps.Stderr, "%s REPL prompt (%s) detected\n", pfx, lp.promptMarker)
				break
			}
			if lp.tickDuringBoot {
				ar.tick(ctx, lp.session) // codex/agy: handle trust prompts during boot
			}
		}
		if !promptSeen {
			fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", pfx, tmuxREPLBootTimeoutS)
			return ExitREPLBootTimeout, nil
		}
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
	artifactSeen := false
	relocErrLogged := false
	attempt := 0
	intervalStart := 0
	intervalBaselinePane, _ := deps.Tmux.CapturePane(ctx, lp.session, 0)
	for elapsed := 0; ; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		if err := ctx.Err(); err != nil {
			// Context cancelled (orchestrator timeout / SIGTERM): stop waiting
			// promptly rather than running out the reviewer's extend budget.
			// Load-bearing once a Stage-1 LLM reviewer can extend at length.
			fmt.Fprintf(deps.Stderr, "%s context cancelled (%v) — abandoning artifact wait\n", pfx, err)
			break
		}
		ready, from, relocErr := artifactReady(cfg)
		if ready {
			artifactSeen = true
			if from != "" {
				fmt.Fprintf(deps.Stderr, "%s artifact relocated from non-canonical %s → %s\n", pfx, from, cfg.Artifact)
			}
			fmt.Fprintf(deps.Stderr, "%s artifact appeared: %s\n", pfx, cfg.Artifact)
			break
		}
		if relocErr != nil && !relocErrLogged {
			// The artifact landed in the non-canonical location but could not be
			// moved (e.g. read-only workspace). Surface it once, immediately,
			// instead of spinning the full wait window with no signal.
			fmt.Fprintf(deps.Stderr, "%s WARN: artifact present at non-canonical path but relocation failed: %v\n", pfx, relocErr)
			relocErrLogged = true
		}
		// Drain live-injection envelopes BEFORE the auto-respond tick so an
		// operator interrupt pre-empts a pending auto-reply on this tick.
		if envs, _ := cursor.Drain(); len(envs) > 0 {
			for _, env := range envs {
				injectEnvelope(ctx, cfg, deps, lp, env)
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
			curPane, _ := deps.Tmux.CapturePane(ctx, lp.session, 0)
			// Progressed = the pane changed during the interval. Stage-0 signal:
			// good for the common cases (growing token counters, new tool calls),
			// but a pure spinner/clock animation also reads as progress — so the
			// maxExtends backstop, not this diff, bounds a spinner-stuck agent
			// (~maxExtends×interval). Stage 1's reviewer inspects StdoutTail to
			// disambiguate genuine work from animation.
			progressed := curPane != intervalBaselinePane
			v := reviewer.Review(StopEvent{
				Kind:       StopArtifactTimeout,
				Phase:      cfg.Agent,
				Cycle:      cfg.Cycle,
				ElapsedS:   elapsed,
				IntervalS:  interval,
				Attempt:    attempt,
				Progressed: progressed,
				StdoutTail: lastLines(curPane, 40),
			})
			fmt.Fprintf(deps.Stderr, "%s stop-review[%s] elapsed=%ds attempt=%d progressed=%v → %s: %s\n",
				pfx, StopArtifactTimeout, elapsed, attempt, progressed, v.Action, v.Reason)
			if v.Action != ReviewExtend {
				break
			}
			attempt++
			intervalStart = elapsed
			intervalBaselinePane = curPane
		}
	}
	if !artifactSeen {
		fmt.Fprintf(deps.Stderr, "%s FAIL: artifact never appeared at %s (stop-review paused after %d interval(s) of %ds)\n", pfx, cfg.Artifact, attempt+1, interval)
		fmt.Fprintf(deps.Stderr, "%s diagnostic: files present under workspace %s:\n", pfx, cfg.Workspace)
		for _, line := range listWorkspaceFiles(cfg.Workspace) {
			fmt.Fprintf(deps.Stderr, "%s   %s\n", pfx, line)
		}
		// TODO(auto-respond slice): write escalation-report.json from final pane.
		return ExitArtifactTimeout, nil
	}

	// --- Capture scrollback: raw → stderr-log, ANSI-stripped → stdout-log.
	raw, _ := deps.Tmux.CapturePane(ctx, lp.session, tmuxArtifactScrollback)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
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
	fmt.Fprintf(deps.Stderr, "%s DONE: artifact-only verdict = SUCCESS\n", pfx)
	return 0, nil
}

// injectEnvelope delivers one live-injection envelope into the running REPL.
// command/nudge/system_rule are idle-gated (injected only when the prompt
// marker is visible); a mid-turn arrival is re-queued, bounded by
// maxInjectDefer. interrupt sends ESC first, then injects regardless of state.
func injectEnvelope(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch, env inbox.Envelope) {
	pfx := "[" + lp.name + "]"
	if env.Kind == inbox.KindInterrupt {
		_ = deps.Tmux.SendKeys(ctx, lp.session, "Escape", false)
		deps.Sleep(injectInterruptSettle)
		injectText(ctx, cfg, deps, lp.session, env.Body)
		fmt.Fprintf(deps.Stderr, "%s injected interrupt (source=%s)\n", pfx, env.Source)
		return
	}

	// Idle-gated kinds: only inject when the agent is waiting at the prompt.
	pane, _ := deps.Tmux.CapturePane(ctx, lp.session, lp.bootScrollback)
	if !strings.Contains(pane, lp.promptMarker) {
		if env.DeferCount >= maxInjectDefer {
			fmt.Fprintf(deps.Stderr, "%s DROP injected %s after %d defers (agent never idled)\n", pfx, env.Kind, env.DeferCount)
			return
		}
		env.DeferCount++
		if err := inbox.Append(cfg.Workspace, cfg.Agent, env, deps.Now); err != nil {
			fmt.Fprintf(deps.Stderr, "%s WARN re-queue of %s failed: %v\n", pfx, env.Kind, err)
		}
		return
	}

	body := env.Body
	if env.Kind == inbox.KindSystemRule {
		body = "## Rules\n" + body
	}
	injectText(ctx, cfg, deps, lp.session, body)
	fmt.Fprintf(deps.Stderr, "%s injected %s (source=%s)\n", pfx, env.Kind, env.Source)
}

// injectText delivers body into the session via the paste buffer (so
// multi-line/special characters survive — SendKeys would mangle them), then
// Enter. It uses a dedicated scratch file so it never collides with the task
// prompt's resolved-prompt.txt.
func injectText(ctx context.Context, cfg *Config, deps Deps, session, body string) {
	scratch := filepath.Join(cfg.Workspace, ".bridge-inbox", orDefault(cfg.Agent, "agent")+"-inject.txt")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch mkdir: %v\n", session, err)
		return
	}
	if err := os.WriteFile(scratch, []byte(body), 0o644); err != nil {
		fmt.Fprintf(deps.Stderr, "[%s] WARN inject scratch write: %v\n", session, err)
		return
	}
	_ = deps.Tmux.LoadBuffer(ctx, session, scratch)
	_ = deps.Tmux.PasteBuffer(ctx, session)
	deps.Sleep(time.Second)
	_ = deps.Tmux.SendKeys(ctx, session, "", true) // Enter
}

// resolveSession returns the tmux session name and whether it is a stable
// named (resume-eligible) session. ephemeralPrefix distinguishes drivers
// (evolve-bridge- / evolve-bridge-codex- / evolve-bridge-agy-). Named
// sessions (claude-tmux only) use evolve-bridge-named-<name>.
func resolveSession(cfg *Config, deps Deps, ephemeralPrefix string) (session string, named bool) {
	if cfg.SessionName != "" {
		return truncate64("evolve-bridge-named-" + cfg.SessionName), true
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

// tmuxCleanup captures final scrollback then kills the session — unless it
// is a named session, which is preserved for resume.
func tmuxCleanup(ctx context.Context, deps Deps, name, session, scrollbackFile string, named bool) {
	pfx := "[" + name + "]"
	if !deps.Tmux.HasSession(ctx, session) {
		return
	}
	if raw, err := deps.Tmux.CapturePane(ctx, session, tmuxArtifactScrollback); err == nil {
		_ = os.WriteFile(scrollbackFile, []byte(raw), 0o644)
	}
	if named {
		fmt.Fprintf(deps.Stderr, "%s session PRESERVED for resume: %s\n", pfx, session)
		return
	}
	_ = deps.Tmux.KillSession(ctx, session)
	fmt.Fprintf(deps.Stderr, "%s session killed: %s\n", pfx, session)
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
