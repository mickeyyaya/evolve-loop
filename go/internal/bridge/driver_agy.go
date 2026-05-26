package bridge

import (
	"context"
	"fmt"
)

// agyDriver is the Antigravity (Gemini-backed) CLI driver — the Go port
// of drivers/agy.sh (`agy -p <prompt> --dangerously-skip-permissions`).
// agy exposes no model flag (all tiers map to its default) and no
// claude-style plan mode, so it rejects permission_mode loudly.
type agyDriver struct{}

func (agyDriver) Name() string { return "agy" }

func (agyDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if cfg.PermissionMode != "" {
		fmt.Fprintf(deps.Stderr, "[agy] permission_mode='%s' is not supported on this CLI\n", cfg.PermissionMode)
		fmt.Fprintln(deps.Stderr, "[agy] Only claude-p and claude-tmux drivers support --permission-mode.")
		fmt.Fprintln(deps.Stderr, "[agy] Agy exposes only --dangerously-skip-permissions; use --allow-bypass + omit permission_mode.")
		return ExitBadFlags, nil
	}
	if cfg.StreamOutput {
		fmt.Fprintln(deps.Stderr, "[agy] NOTE: stream_output=true is not supported on this CLI — no-op (agy has no streaming output flag)")
	}
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "[agy] NOTE: --session-name='%s' is no-op for this driver (single-shot process).\n", cfg.SessionName)
	}

	switch cfg.Model {
	case "haiku", "sonnet", "opus", "auto", "":
		fmt.Fprintf(deps.Stderr, "[agy] tier '%s' → agy default (gemini-3.5-flash); agy has no -m flag\n", cfg.Model)
	default:
		fmt.Fprintf(deps.Stderr, "[agy] WARN: model '%s' is not a Claude tier alias — agy ignores it anyway\n", cfg.Model)
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}
	args := []string{"-p", prompt, "--dangerously-skip-permissions"}
	args = append(args, cfg.ExtraFlags...) // inner-CLI pass-through

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	rc, err := deps.Runner(ctx, resolveBinary(deps, "agy"), args, driverEnv(deps), nil, stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[agy] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[agy] agy exited rc=%d\n", rc)
	return rc, nil
}

func init() { Register(agyDriver{}) }
