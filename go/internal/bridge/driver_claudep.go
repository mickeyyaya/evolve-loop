package bridge

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// claudePArgs builds the `claude -p` argv from cfg and the prepared prompt.
// Pure — extracted from Launch so flag emission is testable without driving a
// real CLI (the same seam as engine.launchArgs).
//
// omittedModel is the model value that was SUPPRESSED, empty when none was.
// An unresolved vocabulary token (isUnresolvedModelToken) must not be sent:
// `claude -p --model top` fails exactly like the cycle-262 `--model auto`
// incident, and this driver builds its own argv, so the realizer's guard never
// saw it. An empty cfg.Model is "nothing requested" rather than "a request we
// refused", so it emits no flag and reports no suppression.
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
		// v0.2 pass-through; bin/bridge already validated the value.
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.StreamOutput {
		// --verbose is required by claude when combining stream-json with -p.
		args = append(args, "--output-format", "stream-json", "--include-partial-messages", "--verbose")
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
	return args, omittedModel
}

// effectiveModelLabel reports the model the CLI will actually run under, for
// logging. A suppressed model must never be logged as if it were dispatched —
// that is how an adversarial-audit tier silently degrades to the account
// default while the log still claims the requested tier.
func effectiveModelLabel(requested, omitted string) string {
	if omitted != "" {
		return ""
	}
	return requested
}

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

	// Workstream B: confine to worktree when this is a source-writing phase
	// and the host can wrap (sandbox-exec / bwrap). Degrades unwrapped.
	name, args, wrapped := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "claude"), args)
	if sandboxRequiredButUnavailable(cfg, wrapped) {
		fmt.Fprintln(deps.Stderr, "[claude-p] safety gate: activated Build explanation contract requires OS sandbox confinement")
		return ExitSafetyGate, nil
	}
	// Publish the agent PID to a per-phase file so the auto-spawn observer's CPU
	// liveness probe can tell a silently-thinking headless agent from a hung one
	// (the tmux drivers use the pane probe instead, so only the headless driver
	// sets this). Derived from StdoutLog so it matches the observer's path
	// (<ws>/<phase>.bridge-pid); a mismatch degrades to no probe (best-effort).
	env := driverEnv(deps)
	if pidFile := core.BridgePIDFile(cfg.StdoutLog); pidFile != "" {
		env = append(env, bridgePidfileEnv+"="+pidFile)
	}
	// cfg.Worktree is "" for non-source-writing phases → inherits caller cwd.
	rc, err := deps.Runner(ctx, name, cfg.Worktree, args, env, nil, stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[claude-p] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[claude-p] claude exited rc=%d\n", rc)
	return rc, nil
}

func init() { Register(claudePDriver{}) }
