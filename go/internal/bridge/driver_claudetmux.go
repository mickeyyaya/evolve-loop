package bridge

import (
	"context"
	"fmt"
)

// claudeTmuxDriver drives an interactive `claude` REPL through tmux — the
// Go port of drivers/claude-tmux.sh. Subscription-billing path (no API
// key). Preflight here (safety gate, cost guards, claude_cmd) then hands
// the shared REPL state machine (runTmuxREPL) the per-driver spec.
type claudeTmuxDriver struct{}

func (claudeTmuxDriver) Name() string { return "claude-tmux" }

func (claudeTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	// Safety gate: require --allow-bypass only when the driver would fall
	// back to --dangerously-skip-permissions (i.e. no permission_mode).
	if !cfg.AllowBypass && cfg.PermissionMode == "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] safety gate: --allow-bypass is required.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] This driver runs claude with --dangerously-skip-permissions inside tmux.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] Alternative: pass --permission-mode=<mode> to use claude's native permission system.")
		return ExitSafetyGate, nil
	}
	if cfg.StreamOutput {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] NOTE: stream_output=true is no-op for this driver — tmux scrollback already streams to stdout-log")
	}

	// Cost-leak guards (the inner REPL inherits ambient env).
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
	if cfg.AnthropicBaseURL != "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] policy bridge.anthropic_base_url set — proxy mode; abort")
		return ExitCostLeak, nil
	}

	session, named := resolveSession(cfg, deps, "evolve-bridge-")

	// Launch flags come from the per-CLI Realization (ADR-0022): model,
	// permission (plan → --permission-mode plan; bypass → --dangerously-skip-
	// permissions), and any claude-keyed raw flags. The Realizer is the single
	// owner, so the flag set never leaks claude argv into agy/codex.
	launchCmd := launchCmdLine(resolveBinary(deps, "claude"), cfg.Realization.LaunchFlags)

	// TODO(manifest slice): prompt marker from the claude-tmux manifest; default ❯.
	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "claude-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmd,
		promptMarker:   tmuxPromptMarkerDefault,
		bootScrollback: 0, // claude renders to the visible pane
		bootIntervalS:  1,
		tickDuringBoot: true, // claude v2.1.193 shows a folder-trust dialog at boot whose ❯ cursor collides with the REPL marker (see manifest trust_prompt)
		exitSeq:        []tmuxKey{{keys: "/exit", enter: true, pauseS: 2}},
		bootOnly:       cfg.BootOnly,
		guardDeadShell: true,
	})
}

func init() { Register(claudeTmuxDriver{}) }
