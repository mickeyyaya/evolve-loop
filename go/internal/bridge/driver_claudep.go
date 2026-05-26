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
	// TODO(credential-isolation slice): port the ANTHROPIC_API_KEY /
	// ANTHROPIC_BASE_URL cost-leak guards (EC_COST_LEAK) with their own tests.
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

	fmt.Fprintf(deps.Stderr, "[claude-p] cycle=%d agent=%s model=%s artifact=%s permission_mode=%s\n",
		cfg.Cycle, cfg.Agent, cfg.Model, cfg.Artifact, orDefault(cfg.PermissionMode, "(default)"))

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	rc, err := deps.Runner(ctx, "claude", args, driverEnv(deps), nil, stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[claude-p] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[claude-p] claude exited rc=%d\n", rc)
	return rc, nil
}

func init() { Register(claudePDriver{}) }
