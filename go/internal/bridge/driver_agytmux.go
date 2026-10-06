package bridge

import "context"

// agyTmuxDriver drives an interactive `agy` (Gemini-backed) TUI through
// tmux — the Go port of drivers/agy-tmux.sh. It has no permission-mode
// and selects its model via the --model launch flag (display-name tokens).
type agyTmuxDriver struct{}

func (agyTmuxDriver) Name() string { return "agy-tmux" }

func (agyTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, handled := tmuxNonClaudePreflight("agy-tmux", cfg, deps); handled {
		return rc, nil
	}
	// No credential-isolation guard: GEMINI_API_KEY/GOOGLE_API_KEY are not
	// on agy's auth path (per drivers/agy-tmux.sh).

	session, named := resolveSession(cfg, deps, "evolve-bridge-agy-")

	return runTmuxREPL(ctx, cfg, deps, agyTmuxLaunch(cfg, deps, session, named))
}

func agyTmuxLaunch(cfg *Config, deps Deps, session string, named bool) tmuxLaunch {
	// Launch flags come from the per-CLI Realization (ADR-0022): agy realizes
	// permission=bypass → --dangerously-skip-permissions and model_tier →
	// --model "<display name>"; launchCmdLine quotes the space/paren tokens.
	// claude-keyed raw flags realize to nothing.
	return tmuxLaunch{
		name:            "agy-tmux",
		session:         session,
		named:           named,
		launchCmd:       launchCmdLine(resolveBinary(deps, "agy"), cfg.Realization.LaunchFlags),
		modelDispatch:   modelDispatchForTmux("agy-tmux", cfg.Realization, cfg.ExtraFlags),
		promptMarker:    "? for shortcuts",
		inputLineMarker: ">",
		bootScrollback:  200, // alt-screen
		bootIntervalS:   2,
		tickDuringBoot:  true, // agy shows a trust prompt during boot
		exitSeq:         []tmuxKey{{keys: "C-c", enter: false, pauseS: 1}, {keys: "C-c", enter: false, pauseS: 1}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	}
}

func init() { Register(agyTmuxDriver{}) }
