package bridge

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func claudePArgs(cfg *Config, prompt string) (args []string, omittedModel string) {
	args = []string{"-p", prompt}
	switch {
	case cfg.Model == "":
	case isUnresolvedModelToken(cfg.Model):
		omittedModel = cfg.Model
	default:
		args = append(args, "--model", cfg.Model)
	}
	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.StreamOutput {
		args = append(args, "--output-format", "stream-json", "--include-partial-messages", "--verbose")
	}
	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, cfg.AllowedTools...)
	}
	args = append(args, cfg.Realization.LaunchFlags...)
	args = append(args, cfg.ExtraFlags...)
	return args, omittedModel
}

func effectiveModelLabel(requested, omitted string) string {
	if omitted != "" {
		return ""
	}
	return requested
}

type claudePDriver struct{}

func (claudePDriver) Name() string { return "claude-p" }

func (claudePDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
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

	args, omittedModel := claudePArgs(cfg, prompt)
	if cfg.SessionName != "" {
		fmt.Fprintf(deps.Stderr, "[claude-p] NOTE: --session-name='%s' is no-op for this driver (single-shot process). Use --cli=claude-tmux for named/resumable sessions.\n", cfg.SessionName)
	}
	if omittedModel != "" {
		fmt.Fprintf(deps.Stderr, "[claude-p] model='%s' is an unresolved tier token → omitting --model (claude picks its default)\n", omittedModel)
	}

	fmt.Fprintf(deps.Stderr, "[claude-p] cycle=%d agent=%s model=%s artifact=%s permission_mode=%s\n",
		cfg.Cycle, cfg.Agent, orDefault(effectiveModelLabel(cfg.Model, omittedModel), "(cli default)"), cfg.Artifact, orDefault(cfg.PermissionMode, "(default)"))

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	name, args, wrapped := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "claude"), args)
	if sandboxRequiredButUnavailable(deps, cfg, wrapped) {
		fmt.Fprintln(deps.Stderr, "[claude-p] safety gate: activated Build explanation contract requires OS sandbox confinement")
		return ExitSafetyGate, nil
	}
	selection := defaultModelDispatch()
	if omittedModel == "" && cfg.Model != "" {
		selection = modelDispatch{model: cfg.Model, source: modelDispatchArgv}
	}
	selection = modelDispatchFromRealization("claude-p", selection, cfg.Realization)
	selection = modelDispatchFromExtraArgs("claude-p", selection, cfg.ExtraFlags)
	observeModelDispatch(deps, selection)
	env := dispatchEnv(deps, cfg)
	if pidFile := core.BridgePIDFile(cfg.StdoutLog); pidFile != "" {
		env = append(env, bridgePidfileEnv+"="+pidFile)
	}
	rc, err := deps.Runner(ctx, name, cfg.Worktree, args, env, nil, stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[claude-p] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[claude-p] claude exited rc=%d\n", rc)
	return rc, nil
}

func init() { Register(claudePDriver{}) }
