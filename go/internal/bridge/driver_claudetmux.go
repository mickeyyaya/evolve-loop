package bridge

import (
	"context"
	"fmt"
)

type claudeTmuxDriver struct{}

func (claudeTmuxDriver) Name() string { return "claude-tmux" }

func (claudeTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if !cfg.AllowBypass && cfg.PermissionMode == "" {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] safety gate: --allow-bypass is required.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] This driver runs claude with --dangerously-skip-permissions inside tmux.")
		fmt.Fprintln(deps.Stderr, "[claude-tmux] Alternative: pass --permission-mode=<mode> to use claude's native permission system.")
		return ExitSafetyGate, nil
	}
	if cfg.StreamOutput {
		fmt.Fprintln(deps.Stderr, "[claude-tmux] NOTE: stream_output=true is no-op for this driver — tmux scrollback already streams to stdout-log")
	}

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

	launchCmd := launchCmdLine(resolveBinary(deps, "claude"), cfg.Realization.LaunchFlags)

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:            "claude-tmux",
		session:         session,
		named:           named,
		launchCmd:       launchCmd,
		modelDispatch:   modelDispatchForTmux("claude-tmux", cfg.Realization, cfg.ExtraFlags),
		promptMarker:    tmuxPromptMarkerDefault,
		inputLineMarker: tmuxPromptMarkerDefault,
		bootScrollback:  0,
		bootIntervalS:   1,
		tickDuringBoot:  true,
		exitSeq:         []tmuxKey{{keys: "/exit", enter: true, pauseS: 2}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	})
}

func init() { Register(claudeTmuxDriver{}) }
