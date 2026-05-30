package bridge

import (
	"context"
	"fmt"
)

// claudePDriver is the headless `claude -p` driver — the Go port of
// drivers/claude-p.sh. It forwards --permission-mode straight into the
// claude argv (claude is the only CLI that supports it).
type claudePDriver struct{}

func (claudePDriver) Name() string { return "claude-p" }

func (claudePDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	// Credential-isolation guards (drivers/claude-p.sh): refuse to run when
	// an ambient auth path would override the CLI's configured one. The
	// in-process inner CLI inherits these via driverEnv, so an ambient leak
	// is real — fail loudly (EC_COST_LEAK) so the operator confirms intent.
	if v, ok := lookupEnv(deps, "ANTHROPIC_API_KEY"); ok && v != "" {
		fmt.Fprintln(deps.Stderr, "[claude-p] credential-isolation guard: ANTHROPIC_API_KEY is set; refusing to run to avoid an ambiguous credential path")
		fmt.Fprintln(deps.Stderr, "[claude-p] unset the variable, or use a different shell, then retry.")
		return ExitCostLeak, nil
	}
	if v, ok := lookupEnv(deps, "ANTHROPIC_BASE_URL"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_ANTHROPIC_BASE_URL"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[claude-p] credential-isolation guard: ANTHROPIC_BASE_URL set without BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1")
			return ExitCostLeak, nil
		}
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}

	args := []string{"-p", prompt, "--model", cfg.Model}
	if cfg.PermissionMode != "" {
		// v0.2 pass-through; bin/bridge already validated the value.
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.StreamOutput {
		// --verbose is required by claude when combining stream-json with -p.
		args = append(args, "--output-format", "stream-json", "--include-partial-messages", "--verbose")
	}
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "[claude-p] NOTE: --session-name='%s' is no-op for this driver (single-shot process). Use --cli=claude-tmux for named/resumable sessions.\n", cfg.SessionName)
	}
	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, cfg.AllowedTools...)
	}
	// Inner-CLI pass-through flags (the bash `--` separator): --bare,
	// --strict-mcp-config, --setting-sources, etc. from the adapter.
	// Profile raw flags (extra_flags_by_cli["claude-p"]) realized per-CLI,
	// then the direct `--` pass-through. Uniform with the tmux drivers.
	args = append(args, cfg.Realization.LaunchFlags...)
	args = append(args, cfg.ExtraFlags...)

	fmt.Fprintf(deps.Stderr, "[claude-p] cycle=%d agent=%s model=%s artifact=%s permission_mode=%s\n",
		cfg.Cycle, cfg.Agent, cfg.Model, cfg.Artifact, orDefault(cfg.PermissionMode, "(default)"))

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	// Workstream B: confine to worktree when this is a source-writing phase
	// and the host can wrap (sandbox-exec / bwrap). Degrades unwrapped.
	name, args := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "claude"), args)
	// cfg.Worktree is "" for non-source-writing phases → inherits caller cwd.
	rc, err := deps.Runner(ctx, name, cfg.Worktree, args, driverEnv(deps), nil, stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[claude-p] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[claude-p] claude exited rc=%d\n", rc)
	return rc, nil
}

func init() { Register(claudePDriver{}) }
