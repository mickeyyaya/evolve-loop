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

	// Launch codex interactively; -m only for a real codex model name
	// (reusing the headless codex driver's mapping — DRY).
	launchCmd := "codex"
	if resolved := mapCodexModel(cfg.Model); isCodexModelName(resolved) {
		launchCmd = "codex -m " + resolved
	}

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "codex-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmd,
		promptMarker:   "›", // U+203A
		bootScrollback: 200, // alt-screen: bare capture-pane is blank
		bootIntervalS:  2,
		exitSeq:        []tmuxKey{{keys: "/quit", enter: true, pauseS: 2}},
	})
}

func init() { Register(codexTmuxDriver{}) }
