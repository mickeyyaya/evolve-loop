package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// claudeTmuxDriver drives an interactive `claude` REPL through tmux — the
// Go port of drivers/claude-tmux.sh. It is the subscription-billing path
// (no API key); the REPL is fed the prompt via the paste buffer and the
// run completes when the artifact file appears.
//
// Slice-1 scope: safety gate, cost-leak guards, session spawn, claude_cmd
// construction (permission-mode swaps OUT --dangerously-skip-permissions),
// REPL-boot wait, prompt delivery, artifact wait, scrollback capture,
// named-session resume/preserve. Deferred to later slices: the
// auto-respond engine (autoRespondTick is a no-op), human-input
// paste-with-review, and manifest-driven prompt markers (default ❯).
type claudeTmuxDriver struct{}

func (claudeTmuxDriver) Name() string { return "claude-tmux" }

const (
	tmuxPromptMarkerDefault = "❯"
	tmuxREPLBootTimeoutS    = 60  // poll iterations @ 1s
	tmuxArtifactTimeoutS    = 300 // deadline; poll @ 2s
	tmuxPaneWidth           = 220
	tmuxPaneHeight          = 80
)

func (claudeTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	// --- Safety gate: require --allow-bypass only when the driver would
	// fall back to --dangerously-skip-permissions (i.e. no permission_mode).
	if !cfg.AllowBypass && cfg.PermissionMode == "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] safety gate: --allow-bypass is required.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] This driver runs claude with --dangerously-skip-permissions inside tmux.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] Alternative: pass --permission-mode=<mode> to use claude's native permission system.")
		return ExitSafetyGate, nil
	}
	if cfg.StreamOutput {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] NOTE: stream_output=true is no-op for this driver — tmux scrollback already streams to stdout-log")
	}

	// --- Cost-leak guards (the inner REPL inherits ambient env).
	if v, ok := lookupEnv(deps, "ANTHROPIC_API_KEY"); ok && v != "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] ANTHROPIC_API_KEY is set — would create an ambiguous credential path; abort")
		return ExitCostLeak, nil
	}
	if v, ok := lookupEnv(deps, "ANTHROPIC_BASE_URL"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_ANTHROPIC_BASE_URL"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[claude-tmux] ANTHROPIC_BASE_URL set without BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1; abort")
			return ExitCostLeak, nil
		}
	}
	if v, ok := lookupEnv(deps, "EVOLVE_ANTHROPIC_BASE_URL"); ok && v != "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] EVOLVE_ANTHROPIC_BASE_URL set — proxy mode; abort")
		return ExitCostLeak, nil
	}

	// TODO(manifest slice): load prompt_marker + interactive_prompts from
	// the claude-tmux manifest; default ❯ for now.
	marker := tmuxPromptMarkerDefault

	workingDir := cfg.Worktree
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}
	if !isDir(workingDir) {
		fmt.Fprintf(deps.Stderr, "[claude-tmux] working dir does not exist: %s\n", workingDir)
		return ExitBadFlags, nil
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}
	if _, _, closeFn, err := openDriverLogs(cfg); err == nil {
		closeFn() // ensure dirs exist; the tmux driver writes logs itself at the end
	}
	resolvedPromptFile := filepath.Join(cfg.Workspace, "resolved-prompt.txt")
	if err := os.WriteFile(resolvedPromptFile, []byte(prompt+"\n"), 0o644); err != nil {
		return ExitBadFlags, fmt.Errorf("[claude-tmux] write resolved prompt: %w", err)
	}

	session, named := resolveSession(cfg, deps)
	namedExists := false
	if named {
		if deps.Tmux.HasSession(ctx, session) {
			namedExists = true
			fmt.Fprintf(deps.Stderr, "[claude-tmux] RESUME: reattaching to existing named session '%s'\n", session)
		} else {
			fmt.Fprintf(deps.Stderr, "[claude-tmux] CREATE-NAMED: new named session '%s' (persists on exit for resume)\n", session)
		}
	}
	scrollbackFile := filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")
	fmt.Fprintf(deps.Stderr, "[claude-tmux] session=%s model=%s workdir=%s\n", session, cfg.Model, workingDir)
	defer tmuxCleanup(ctx, deps, session, scrollbackFile, named)

	// --- Spawn session + cd (skip when resuming a live named session).
	if !namedExists {
		if err := deps.Tmux.NewSession(ctx, session, tmuxPaneWidth, tmuxPaneHeight); err != nil {
			return ExitBadFlags, fmt.Errorf("[claude-tmux] new-session: %w", err)
		}
		deps.Sleep(time.Second)
		_ = deps.Tmux.SendKeys(ctx, session, "cd "+workingDir, true)
		deps.Sleep(time.Second)
	}

	// --- claude_cmd: permission_mode swaps OUT --dangerously-skip-permissions
	// (plan mode in particular is incompatible with bypass).
	var claudeCmd string
	if cfg.PermissionMode != "" {
		claudeCmd = fmt.Sprintf("claude --model %s --permission-mode %s", cfg.Model, cfg.PermissionMode)
	} else {
		claudeCmd = fmt.Sprintf("claude --model %s --dangerously-skip-permissions", cfg.Model)
	}

	if !namedExists {
		_ = deps.Tmux.SendKeys(ctx, session, claudeCmd, true)
		fmt.Fprintf(deps.Stderr, "[claude-tmux] launching: %s\n", claudeCmd)

		// --- Wait for the REPL prompt marker.
		promptSeen := false
		for elapsed := 0; elapsed < tmuxREPLBootTimeoutS; elapsed++ {
			deps.Sleep(time.Second)
			pane, _ := deps.Tmux.CapturePane(ctx, session, 0)
			if strings.Contains(pane, marker) {
				promptSeen = true
				fmt.Fprintf(deps.Stderr, "[claude-tmux] REPL prompt (%s) detected\n", marker)
				break
			}
		}
		if !promptSeen {
			fmt.Fprintf(deps.Stderr, "[claude-tmux] FAIL: REPL prompt never appeared after %ds\n", tmuxREPLBootTimeoutS)
			return ExitREPLBootTimeout, nil
		}
	}

	// --- Deliver the prompt via the paste buffer.
	// TODO(human-input slice): paste-with-review keystroke timing when active.
	_ = deps.Tmux.LoadBuffer(ctx, session, resolvedPromptFile)
	_ = deps.Tmux.PasteBuffer(ctx, session)
	deps.Sleep(time.Second)
	_ = deps.Tmux.SendKeys(ctx, session, "", true) // Enter
	fmt.Fprintln(deps.Stderr, "[claude-tmux] prompt delivered")

	// --- Wait for the artifact, ticking the auto-respond fallback.
	deadline := tmuxArtifactTimeoutS
	artifactSeen := false
	for elapsed := 0; elapsed < deadline; elapsed += 2 {
		deps.Sleep(2 * time.Second)
		if fileNonEmpty(cfg.Artifact) {
			artifactSeen = true
			fmt.Fprintf(deps.Stderr, "[claude-tmux] artifact appeared: %s\n", cfg.Artifact)
			break
		}
		action, rc := autoRespondTick(ctx, deps, session)
		switch rc {
		case 0, 1: // noop / responded — keep polling
		case 2: // extend_timeout
			if secs := parseExtendSecs(action); secs > 0 {
				deadline += secs
				fmt.Fprintf(deps.Stderr, "[claude-tmux] artifact-poll deadline extended +%ds → %ds\n", secs, deadline)
			}
		case 85:
			fmt.Fprintln(deps.Stderr, "[claude-tmux] auto-respond escalation; abandoning run")
			return ExitUnknownPrompt, nil
		case 86:
			fmt.Fprintln(deps.Stderr, "[claude-tmux] auto-respond loop guard tripped; abandoning run")
			return ExitRespondLoopGuard, nil
		}
	}
	if !artifactSeen {
		fmt.Fprintf(deps.Stderr, "[claude-tmux] FAIL: artifact never appeared at %s after %ds\n", cfg.Artifact, deadline)
		// TODO(auto-respond slice): write escalation-report.json from the final pane.
		return ExitArtifactTimeout, nil
	}

	// --- Capture scrollback: raw → stderr-log, ANSI-stripped → stdout-log.
	raw, _ := deps.Tmux.CapturePane(ctx, session, 10000)
	_ = os.WriteFile(cfg.StderrLog, []byte(raw+"\n"), 0o644)
	_ = os.WriteFile(cfg.StdoutLog, []byte(stripANSI(raw)+"\n"), 0o644)
	fmt.Fprintln(deps.Stderr, "[claude-tmux] scrollback captured")

	if named {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] RESUME-PRESERVE: skipping /exit; claude REPL stays running for next launch")
	} else {
		_ = deps.Tmux.SendKeys(ctx, session, "/exit", true)
		deps.Sleep(2 * time.Second)
	}
	fmt.Fprintln(deps.Stderr, "[claude-tmux] DONE: artifact-only verdict = SUCCESS")
	return 0, nil
}

// resolveSession returns the tmux session name and whether it is a stable
// named (resume-eligible) session. Named: evolve-bridge-named-<name>;
// ephemeral: evolve-bridge-c<cycle>-<agent>-pid<pid>-<unix>. Truncated to 64.
func resolveSession(cfg *Config, deps Deps) (session string, named bool) {
	if cfg.SessionName != "" {
		s := "evolve-bridge-named-" + cfg.SessionName
		return truncate64(s), true
	}
	agent := orDefault(cfg.Agent, "probe")
	s := fmt.Sprintf("evolve-bridge-c%d-%s-pid%d-%d", cfg.Cycle, agent, os.Getpid(), deps.Now().Unix())
	return truncate64(s), false
}

func truncate64(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

// autoRespondTick is the M4-slice-1 no-op placeholder for the auto-respond
// engine (lib/auto-respond.sh). rc: 0=noop, 1=responded, 2=extend_timeout
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
func tmuxCleanup(ctx context.Context, deps Deps, session, scrollbackFile string, named bool) {
	if !deps.Tmux.HasSession(ctx, session) {
		return
	}
	if raw, err := deps.Tmux.CapturePane(ctx, session, 10000); err == nil {
		_ = os.WriteFile(scrollbackFile, []byte(raw), 0o644)
	}
	if named {
		fmt.Fprintf(deps.Stderr, "[claude-tmux] session PRESERVED for resume: %s\n", session)
		return
	}
	_ = deps.Tmux.KillSession(ctx, session)
	fmt.Fprintf(deps.Stderr, "[claude-tmux] session killed: %s\n", session)
}

func init() { Register(claudeTmuxDriver{}) }
