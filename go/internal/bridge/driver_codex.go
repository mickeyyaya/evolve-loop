package bridge

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

// codexDriver is the OpenAI Codex CLI driver — the Go port of
// drivers/codex.sh (`codex exec --output-last-message`). Codex has no
// claude-style plan mode, so it rejects permission_mode loudly rather
// than silently ignoring an operator's safety declaration.
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
	// Credential-isolation guard (drivers/codex.sh): an ambient
	// OPENAI_API_KEY would be inherited by the in-process inner CLI.
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

	resolved := mapCodexModel(cfg.Model)
	args := []string{"exec", "--output-last-message", cfg.Artifact}
	switch {
	case resolved == "" || resolved == "auto":
		fmt.Fprintf(deps.Stderr, "[codex] model='%s' → omitting -m (codex picks default)\n", cfg.Model)
	case isCodexModelName(resolved):
		args = []string{"exec", "-m", resolved, "--output-last-message", cfg.Artifact}
		fmt.Fprintf(deps.Stderr, "[codex] model: %s → %s (via -m)\n", cfg.Model, resolved)
	default:
		fmt.Fprintf(deps.Stderr, "[codex] WARN: unrecognized model '%s' — omitting -m\n", resolved)
	}
	args = append(args, cfg.Realization.LaunchFlags...) // profile raw flags (extra_flags_by_cli["codex"])
	args = append(args, cfg.ExtraFlags...)              // direct `--` pass-through

	stdoutF, stderrF, closeFn, err := openDriverLogs(cfg)
	if err != nil {
		return ExitBadFlags, err
	}
	defer closeFn()

	// codex reads the prompt on stdin.
	// Workstream B: sandbox-confine source-writing phases (CLI-agnostic).
	name, args := wrapHeadlessInvocation(deps, cfg, resolveBinary(deps, "codex"), args)
	// cfg.Worktree is "" for non-source-writing phases → inherits caller cwd.
	rc, err := deps.Runner(ctx, name, cfg.Worktree, args, driverEnv(deps), bytes.NewReader([]byte(prompt)), stdoutF, stderrF)
	if err != nil {
		return ExitMissingBinary, fmt.Errorf("[codex] %w", err)
	}
	fmt.Fprintf(deps.Stderr, "[codex] codex exited rc=%d\n", rc)
	return rc, nil
}

// mapCodexModel maps a tier to a codex model name (researched 2026-05-21 per
// drivers/codex.sh). It accepts BOTH the canonical tier vocabulary
// (fast/balanced/deep — what policy pins and `evolve setup apply` store) and
// the legacy Claude aliases (haiku/sonnet/opus); each tier resolves to the same
// native id. Native ids and genuinely unknown values pass through unchanged.
// (cycle-378: a pinned canonical tier "deep" was previously unrecognized here,
// so -m was omitted and codex exited rc=1.)
func mapCodexModel(m string) string {
	switch m {
	case "fast", "haiku":
		return "gpt-5.4-mini"
	case "balanced", "sonnet":
		return "gpt-5.4"
	case "deep", "opus":
		return "gpt-5.5"
	}
	return m
}

// isCodexModelName reports whether m looks like a codex-acceptable model
// id (the prefixes drivers/codex.sh passes via -m).
func isCodexModelName(m string) bool {
	for _, p := range []string{"gpt-", "o-", "o1", "o3", "o4", "codex"} {
		if strings.HasPrefix(m, p) {
			return true
		}
	}
	return false
}

func init() { Register(codexDriver{}) }
