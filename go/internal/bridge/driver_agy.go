package bridge

import (
	"context"
	"fmt"
	"strings"
)

type agyDriver struct{}

func (agyDriver) Name() string { return "agy" }

func (agyDriver) DeepProbeArgs(m Manifest) []string {
	return headlessProbeArgs(m)
}

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

	logAgyRoute(deps, cfg)

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}
	args := []string{"-p", prompt, "--dangerously-skip-permissions"}
	args = append(args, cfg.Realization.LaunchFlags...)
	args = append(args, cfg.ExtraFlags...)
	name, args, wrapped := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "agy"), args)
	if sandboxRequiredButUnavailable(deps, cfg, wrapped) {
		fmt.Fprintln(deps.Stderr, "[agy] required OS sandbox is unavailable; refusing an unconfined launch")
		return ExitSafetyGate, nil
	}
	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	observeModelDispatch(deps, defaultModelDispatch())
	rc, err := deps.runDispatch(ctx, cfg, dispatchRun{name: name, args: args, stdout: stdoutF, stderr: stderrF})
	if err != nil {
		return rc, fmt.Errorf("[agy] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[agy] agy exited rc=%d\n", rc)
	return rc, nil
}

func logAgyRoute(deps Deps, cfg *Config) {
	flags := agyRouteFlags(cfg.Realization.LaunchFlags)
	if len(flags) == 0 {
		fmt.Fprintf(deps.Stderr, "[agy] tier '%s' → no model flag in the route; agy uses its own default model\n", cfg.Model)
		return
	}
	fmt.Fprintf(deps.Stderr, "[agy] tier '%s' → route flags: %s\n", cfg.Model, strings.Join(flags, " "))
}

func agyRouteFlags(launchFlags []string) []string {
	var flags []string
	for i, tok := range launchFlags {
		if (tok == "--model" || tok == "--effort") && i+1 < len(launchFlags) {
			flags = append(flags, tok, launchFlags[i+1])
		}
	}
	return flags
}

func init() { Register(agyDriver{}) }
