package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	exitSeq        []tmuxKey // keystrokes to close the REPL cleanly
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
			autoRespondTick(ctx, deps, lp.session) // handle trust/auth prompts during boot
		}
		if !promptSeen {
			fmt.Fprintf(deps.Stderr, "%s FAIL: REPL prompt never appeared after %ds\n", pfx, tmuxREPLBootTimeoutS)
			return ExitREPLBootTimeout, nil
		}
	}

	// --- Deliver the prompt via the paste buffer.
	// TODO(human-input slice): paste-with-review keystroke timing when active.
	_ = deps.Tmux.LoadBuffer(ctx, lp.session, resolvedPromptFile)
	_ = deps.Tmux.PasteBuffer(ctx, lp.session)
	deps.Sleep(time.Second)
	_ = deps.Tmux.SendKeys(ctx, lp.session, "", true) // Enter
	fmt.Fprintf(deps.Stderr, "%s prompt delivered\n", pfx)

	// --- Wait for the artifact, ticking the auto-respond fallback.
	deadline := tmuxArtifactTimeoutS
	artifactSeen := false
	for elapsed := 0; elapsed < deadline; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		if fileNonEmpty(cfg.Artifact) {
			artifactSeen = true
			fmt.Fprintf(deps.Stderr, "%s artifact appeared: %s\n", pfx, cfg.Artifact)
			break
		}
		action, rc := autoRespondTick(ctx, deps, lp.session)
		switch rc {
		case 0, 1: // noop / responded
		case 2:
			if secs := parseExtendSecs(action); secs > 0 {
				deadline += secs
				fmt.Fprintf(deps.Stderr, "%s artifact-poll deadline extended +%ds → %ds\n", pfx, secs, deadline)
			}
		case 85:
			fmt.Fprintf(deps.Stderr, "%s auto-respond escalation; abandoning run\n", pfx)
			return ExitUnknownPrompt, nil
		case 86:
			fmt.Fprintf(deps.Stderr, "%s auto-respond loop guard tripped; abandoning run\n", pfx)
			return ExitRespondLoopGuard, nil
		}
	}
	if !artifactSeen {
		fmt.Fprintf(deps.Stderr, "%s FAIL: artifact never appeared at %s after %ds\n", pfx, cfg.Artifact, deadline)
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

// autoRespondTick is the placeholder for the auto-respond engine
// (lib/auto-respond.sh). rc: 0=noop, 1=responded, 2=extend_timeout
// (action "extend:<secs>"), 85=escalate, 86=loop-guard. Replaced by the
// real engine in the auto-respond slice.
func autoRespondTick(_ context.Context, _ Deps, _ string) (string, int) {
	return "", 0
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
