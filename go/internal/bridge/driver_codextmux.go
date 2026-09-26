package bridge

import (
	"context"
	"fmt"
)

// codexTmuxDriver drives an interactive `codex` TUI through tmux. codex uses
// alt-screen rendering (boot wait must read scrollback, not the visible
// pane) and the › prompt marker; it has no permission-mode and no
// named-session support.
type codexTmuxDriver struct{}

func (codexTmuxDriver) Name() string { return "codex-tmux" }

// Preflight pre-trusts cfg.Worktree and cfg.Workspace in ~/.codex/config.toml
// so codex's own permission layer doesn't render the workspace-write modal.
// A returned error is logged by Engine.Launch and does not abort the phase.
func (codexTmuxDriver) Preflight(ctx context.Context, cfg *Config, deps Deps) error {
	_ = ctx // reserved for future timeouts on TOML rewrites
	_ = deps
	if err := pretrustCodexProjects(cfg); err != nil {
		return err
	}
	return dismissCodexUpdateNag()
}

func (codexTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, handled := tmuxNonClaudePreflight("codex-tmux", cfg, deps); handled {
		return rc, nil
	}
	// Credential-isolation guard: runs after the safety gate above.
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[codex-tmux] credential-isolation guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1")
			return ExitCostLeak, nil
		}
	}

	session, named := resolveSession(cfg, deps, "evolve-bridge-codex-")

	// Launch flags come from the per-CLI Realization (ADR-0022): codex resolves
	// the model tier via its manifest model_tier_map and emits it as -m;
	// permission is a controller no-op (trust handled by the auto-responder).
	// No claude argv reaches codex.
	flags := cfg.Realization.LaunchFlags
	dispatched := modelDispatchFromRealization("codex-tmux", defaultModelDispatch(), cfg.Realization)
	// Clamps the model to a ChatGPT-safe one on subscription auth: a model
	// outside chatgpt_safe_models is 400-rejected on ChatGPT accounts, hanging
	// the phase on an undismissable model-switch modal. A manifest load
	// failure leaves flags untouched.
	if m, err := LoadManifest("codex-tmux"); err == nil {
		if clamped, from, to := clampCodexModelForAuth(flags, m, codexAuthMode(deps)); from != "" {
			flags = clamped
			if !dispatched.uncertain {
				dispatched = modelDispatchFromArgs("codex-tmux", defaultModelDispatch(), flags)
			}
			fmt.Fprintf(deps.Stderr, "[codex-tmux] model clamp (auth=chatgpt): %s → %s (not in the manifest's chatgpt_safe_models for a ChatGPT account)\n", from, to)
		}
	}
	dispatched = modelDispatchFromExtraArgs("codex-tmux", dispatched, cfg.ExtraFlags)
	launchCmd := launchCmdLine(resolveBinary(deps, "codex"), flags)

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:          "codex-tmux",
		session:       session,
		named:         named,
		launchCmd:     launchCmd,
		modelDispatch: dispatched,
		promptMarker:  "›", // U+203A
		// Same glyph: codex's boot-ready marker IS its input-line prompt.
		inputLineMarker: "›",
		bootScrollback:  200, // alt-screen: bare capture-pane is blank
		bootIntervalS:   2,
		tickDuringBoot:  true, // codex shows a trust prompt during boot
		bootMenuSkip:    "2",  // codex update menu: Skip before prompt injection
		exitSeq:         []tmuxKey{{keys: "/quit", enter: true, pauseS: 2}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	})
}

func init() { Register(codexTmuxDriver{}) }
