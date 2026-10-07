package bridge

import (
	"context"
	"fmt"
)

type agyTmuxDriver struct{ target string }

func (d agyTmuxDriver) Name() string { return d.target }

func (d agyTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if rc, handled := tmuxNonClaudePreflight(d.target, cfg, deps); handled {
		return rc, nil
	}
	// No credential-isolation guard: GEMINI_API_KEY/GOOGLE_API_KEY are not
	// on agy's auth path (per drivers/agy-tmux.sh).

	session, named := resolveSession(cfg, deps, "evolve-bridge-agy-")

	check, err := d.launchModelCheck()
	if err != nil {
		return ExitBadFlags, fmt.Errorf("[%s] launch model check: %w", d.target, err)
	}
	return runTmuxREPL(ctx, cfg, deps, agyTmuxLaunch(d.target, cfg, deps, session, named).withModelCheck(check))
}

func (d agyTmuxDriver) launchModelCheck() (launchModelCheck, error) {
	return launchModelCheckFor(d.target)
}

func agyTmuxLaunch(target string, cfg *Config, deps Deps, session string, named bool) tmuxLaunch {
	// Launch flags come from the per-CLI Realization (ADR-0022): agy realizes
	// permission=bypass → --dangerously-skip-permissions and model_tier →
	// --model "<display name>"; launchCmdLine quotes the space/paren tokens.
	// claude-keyed raw flags realize to nothing.
	return tmuxLaunch{
		name:            target,
		session:         session,
		named:           named,
		launchCmd:       launchCmdLine(resolveBinary(deps, "agy"), cfg.Realization.LaunchFlags),
		modelDispatch:   modelDispatchForTmux(target, cfg.Realization, cfg.ExtraFlags),
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

func init() {
	Register(agyTmuxDriver{target: "agy-tmux"})
	Register(agyTmuxDriver{target: "agy-claude-tmux"})
}
