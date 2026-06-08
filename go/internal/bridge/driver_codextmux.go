package bridge

import (
	"context"
	"fmt"
)

// codexTmuxDriver drives an interactive `codex` TUI through tmux — the Go
// port of drivers/codex-tmux.sh. codex uses alt-screen rendering (boot
// wait must read scrollback, not the visible pane) and the › prompt
// marker; it has no permission-mode and no named-session support.
type codexTmuxDriver struct{}

func (codexTmuxDriver) Name() string { return "codex-tmux" }

// Preflight pre-trusts cfg.Worktree + cfg.Workspace in ~/.codex/config.toml so
// codex's own permission layer doesn't render the runtime workspace-write
// modal that hung cycle-122 tdd (incident report + research dossier codex
// Fix A). This was an inline call at the top of Launch until cycle-124 G3
// promoted it through the optional CLIPreflight interface (driver.go). Same
// best-effort semantics — a returned error is logged by Engine.Launch and
// does NOT abort the phase (Fix 2's extended fallback trigger list defends
// downstream). ctx + deps are retained for future codex-specific prep work
// (binary-version probe, OAuth refresh) that may need them; the current
// implementation only reads cfg.
func (codexTmuxDriver) Preflight(ctx context.Context, cfg *Config, deps Deps) error {
	_ = ctx // reserved for future timeouts on TOML rewrites
	_ = deps
	return pretrustCodexProjects(cfg)
}

func (codexTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, handled := tmuxNonClaudePreflight("codex-tmux", cfg, deps); handled {
		return rc, nil
	}
	// Credential-isolation guard (after the safety gate, per codex-tmux.sh).
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[codex-tmux] credential-isolation guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1")
			return ExitCostLeak, nil
		}
	}

	session, named := resolveSession(cfg, deps, "evolve-bridge-codex-")

	// Launch flags come from the per-CLI Realization (ADR-0022): codex resolves
	// the model tier via its manifest model_tier_map (sonnet → gpt-5.4) and
	// emits it as -m; permission is a controller no-op (trust handled by the
	// auto-responder). No claude argv reaches codex.
	flags := cfg.Realization.LaunchFlags
	// cycle-142: clamp the model to a ChatGPT-safe one on subscription auth.
	// gpt-5.4/gpt-5.5 are 400-rejected on ChatGPT accounts (API-key-only by
	// plan tier), which otherwise hangs the phase on an undismissable
	// model-switch modal. Best-effort: a manifest load failure leaves flags
	// untouched (the legacy behavior).
	if m, err := LoadManifest("codex-tmux"); err == nil {
		if clamped, from, to := clampCodexModelForAuth(flags, m, codexAuthMode(deps)); from != "" {
			flags = clamped
			fmt.Fprintf(deps.Stderr, "[codex-tmux] model clamp (auth=chatgpt): %s → %s (gpt-5.4/gpt-5.5 are not usable on a ChatGPT account)\n", from, to)
		}
	}
	launchCmd := launchCmdLine(resolveBinary(deps, "codex"), flags)

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "codex-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmd,
		promptMarker:   "›", // U+203A
		bootScrollback: 200, // alt-screen: bare capture-pane is blank
		bootIntervalS:  2,
		tickDuringBoot: true, // codex shows a trust prompt during boot
		exitSeq:        []tmuxKey{{keys: "/quit", enter: true, pauseS: 2}},
		bootOnly:       cfg.BootOnly,
	})
}

func init() { Register(codexTmuxDriver{}) }
