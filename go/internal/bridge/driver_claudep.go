package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func claudePArgs(cfg *Config, prompt string) []string {
	args := []string{"-p", prompt}
	if cfg.StreamOutput {
		args = append(args, "--output-format", "stream-json", "--include-partial-messages", "--verbose")
	}
	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, cfg.AllowedTools...)
	}
	args = append(args, cfg.Realization.LaunchFlags...)
	args = append(args, cfg.ExtraFlags...)
	return args
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
		if !deps.AllowAnthropicBaseURL {
			fmt.Fprintln(deps.Stderr, "[claude-p] credential-isolation guard: ANTHROPIC_BASE_URL set without policy bridge.allow_anthropic_base_url")
			return ExitCostLeak, nil
		}
	}

	prompt, err := preparePrompt(cfg, deps)
	if err != nil {
		return ExitBadFlags, err
	}

	args := claudePArgs(cfg, prompt)
	omittedModel := cfg.Realization.ModelOmitted
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
	if err := writeSystemPromptFile(cfg); err != nil {
		return ExitBadFlags, fmt.Errorf("[claude-p] write system prompt: %w", err)
	}

	name, args, wrapped := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "claude"), args)
	if sandboxRequiredButUnavailable(deps, cfg, wrapped) {
		fmt.Fprintln(deps.Stderr, "[claude-p] safety gate: activated Build explanation contract requires OS sandbox confinement")
		return ExitSafetyGate, nil
	}
	observeModelDispatch(deps, modelDispatchForTmux("claude-p", cfg.Realization, cfg.ExtraFlags))
	rc, idled, err := runIdleWatched(ctx, cfg, deps, dispatchRun{name: name, args: args, stdout: stdoutF, stderr: stderrF, extraEnv: pidfileEnvOf(cfg)})
	if idled {
		fmt.Fprintln(deps.Stderr, launchoutcome.IdleOutputLine(int(headlessIdleBudget(cfg, deps)/time.Second)))
		return ExitArtifactTimeout, nil
	}
	if err != nil {
		return rc, fmt.Errorf("[claude-p] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[claude-p] claude exited rc=%d\n", rc)
	return classifyHeadlessExit(rc, "claude-p", cfg, deps), nil
}

func pidfileEnvOf(cfg *Config) []string {
	if pidFile := core.BridgePIDFile(cfg.StdoutLog); pidFile != "" {
		return []string{bridgePidfileEnv + "=" + pidFile}
	}
	return nil
}

func writeSystemPromptFile(cfg *Config) error {
	_, err := stateAuthority(cfg.Realization.SystemPromptFile)
	return err
}

func classifyHeadlessExit(rc int, cli string, cfg *Config, deps Deps) int {
	if rc == ExitOK {
		return rc
	}
	m, err := LoadManifest(cli)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[claude-p] WARN: manifest unavailable (%v) — exit not classified\n", err)
		return rc
	}
	return classifyWithSpec(rc, m.HeadlessOutcome, cfg, deps)
}

func classifyWithSpec(rc int, spec *HeadlessOutcomeSpec, cfg *Config, deps Deps) int {
	pattern, err := spec.Classify(headlessEvidenceFor(cfg))
	if err != nil {
		fmt.Fprintf(deps.Stderr, "[claude-p] WARN: headless outcome rules unusable (%v) — exit not classified\n", err)
		return rc
	}
	if pattern == "" {
		return rc
	}
	fmt.Fprintln(deps.Stderr, launchoutcome.EscalationLine(pattern, "escalate"))
	return ExitUnknownPrompt
}

func init() { Register(claudePDriver{}) }
