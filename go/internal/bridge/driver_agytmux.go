package bridge

import "context"

// agyTmuxDriver drives an interactive `agy` (Gemini-backed) TUI through
// tmux — the Go port of drivers/agy-tmux.sh. agy 1.0.15 selects its model
// via the --model launch flag (display-name tokens, cycle-447 probe); it has
// no permission-mode, renders alt-screen (boot wait reads scrollback), its
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
	// permission=bypass → --dangerously-skip-permissions and model_tier →
	// --model "<display name>" (agy 1.0.15, cycle-447; launchCmdLine quotes
	// the space/paren tokens). claude-keyed raw flags realize to nothing.
	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "agy-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmdLine(resolveBinary(deps, "agy"), cfg.Realization.LaunchFlags),
		promptMarker:   "? for shortcuts",
		bootScrollback: 200, // alt-screen
		bootIntervalS:  2,
		tickDuringBoot: true, // agy shows a trust prompt during boot
		// agy quits on Ctrl+C twice (no Enter).
		exitSeq:        []tmuxKey{{keys: "C-c", enter: false, pauseS: 1}, {keys: "C-c", enter: false, pauseS: 1}},
		bootOnly:       cfg.BootOnly,
		guardDeadShell: true,
	})
}

func init() { Register(agyTmuxDriver{}) }
