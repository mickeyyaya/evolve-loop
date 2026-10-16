package bridge

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

type codexDriver struct{}

func (codexDriver) Name() string { return "codex" }

func (codexDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if cfg.PermissionMode != "" {
		fmt.Fprintf(deps.Stderr, "[codex] permission_mode='%s' is not supported on this CLI\n", cfg.PermissionMode)
		fmt.Fprintln(deps.Stderr, "[codex] Only claude-p and claude-tmux drivers support --permission-mode.")
		fmt.Fprintln(deps.Stderr, "[codex] For codex, use --sandbox <mode> via the prompt or omit permission_mode.")
		return ExitBadFlags, nil
	}
	if cfg.StreamOutput {
		fmt.Fprintln(deps.Stderr, "[codex] NOTE: stream_output=true is not supported on this CLI — no-op (codex has no streaming output flag)")
	}
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "[codex] NOTE: --session-name='%s' is no-op for this driver (single-shot process).\n", cfg.SessionName)
	}
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[codex] credential-isolation guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1")
			return ExitCostLeak, nil
		}
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}

	man, merr := LoadManifest(cfg.CLI)
	if merr != nil {
		fmt.Fprintf(deps.Stderr, "[codex] WARN: manifest unavailable (%v) — tier not translated\n", merr)
	}
	resolved := resolveTierModel(man, cfg.Model)
	args := []string{"exec", "--output-last-message", cfg.Artifact}
	switch {
	case resolved == "" || isUnresolvedModelToken(resolved):
		fmt.Fprintf(deps.Stderr, "[codex] model='%s' → omitting -m (codex picks default)\n", cfg.Model)
	case isCodexModelName(resolved):
		args = []string{"exec", "-m", resolved, "--output-last-message", cfg.Artifact}
		fmt.Fprintf(deps.Stderr, "[codex] model: %s → %s (via -m)\n", cfg.Model, resolved)
	default:
		fmt.Fprintf(deps.Stderr, "[codex] WARN: unrecognized model '%s' — omitting -m\n", resolved)
	}
	if !argsContainEffort(cfg.Realization.LaunchFlags) && !argsContainEffort(cfg.ExtraFlags) {
		args = append(args, "-c", "model_reasoning_effort=high")
	}
	args = append(args, cfg.Realization.LaunchFlags...)
	args = append(args, cfg.ExtraFlags...)

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	name, args, wrapped := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "codex"), args)
	if sandboxRequiredButUnavailable(deps, cfg, wrapped) {
		fmt.Fprintln(deps.Stderr, "[codex] safety gate: activated Build explanation contract requires OS sandbox confinement")
		return ExitSafetyGate, nil
	}
	selection := defaultModelDispatch()
	if resolved != "" && !isUnresolvedModelToken(resolved) && isCodexModelName(resolved) {
		selection = modelDispatch{model: resolved, source: modelDispatchArgv}
	}
	selection = modelDispatchFromRealization("codex", selection, cfg.Realization)
	selection = modelDispatchFromExtraArgs("codex", selection, cfg.ExtraFlags)
	observeModelDispatch(deps, selection)
	rc, err := deps.Runner(ctx, name, cfg.Worktree, args, dispatchEnv(deps, cfg), bytes.NewReader([]byte(prompt)), stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[codex] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[codex] codex exited rc=%d\n", rc)
	return rc, nil
}

func isCodexModelName(m string) bool {
	for _, p := range []string{"gpt-", "o-", "o1", "o3", "o4", "codex"} {
		if strings.HasPrefix(m, p) {
			return true
		}
	}
	return false
}

func init() { Register(codexDriver{}) }

func argsContainEffort(args []string) bool {
	for _, a := range args {
		if strings.Contains(a, "model_reasoning_effort=") {
			return true
		}
	}
	return false
}
