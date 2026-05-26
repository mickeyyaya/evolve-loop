package bridge

import "context"

// agyTmuxDriver drives an interactive `agy` (Gemini-backed) TUI through
// tmux — the Go port of drivers/agy-tmux.sh. agy has no model flag and no
// permission-mode; it renders alt-screen (boot wait reads scrollback), its
// ready marker is the "? for shortcuts" footer, and it exits via Ctrl+C ×2.
type agyTmuxDriver struct{}

func (agyTmuxDriver) Name() string { return "agy-tmux" }

func (agyTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, handled := tmuxNonClaudePreflight("agy-tmux", cfg, deps); handled {
		return rc, nil
	}
	// No credential-isolation guard: GEMINI_API_KEY/GOOGLE_API_KEY are not
	// on agy's auth path (per drivers/agy-tmux.sh).

	session, named := resolveSession(cfg, deps, "evolve-bridge-agy-")

	// Launch flags come from the per-CLI Realization (ADR-0022): agy realizes
	// permission=bypass → --dangerously-skip-permissions; model tier is a no-op
	// (agy has no -m flag) and claude-keyed raw flags realize to nothing.
	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "agy-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmdLine("agy", cfg.Realization.LaunchFlags),
		promptMarker:   "? for shortcuts",
		bootScrollback: 200, // alt-screen
		bootIntervalS:  2,
		tickDuringBoot: true, // agy shows a trust prompt during boot
		// agy quits on Ctrl+C twice (no Enter).
		exitSeq: []tmuxKey{{keys: "C-c", enter: false, pauseS: 1}, {keys: "C-c", enter: false, pauseS: 1}},
	})
}

func init() { Register(agyTmuxDriver{}) }
