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
	return tmuxLaunch{
		name:            target,
		session:         session,
		named:           named,
		launchCmd:       launchCmdLine(resolveBinary(deps, "agy"), cfg.Realization.LaunchFlags),
		modelDispatch:   modelDispatchForTmux(target, cfg.Realization, cfg.ExtraFlags),
		promptMarker:    "? for shortcuts",
		inputLineMarker: ">",
		bootScrollback:  200,
		bootIntervalS:   2,
		tickDuringBoot:  true,
		exitSeq:         []tmuxKey{{keys: "C-c", enter: false, pauseS: 1}, {keys: "C-c", enter: false, pauseS: 1}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	}
}

func init() {
	Register(agyTmuxDriver{target: "agy-tmux"})
	Register(agyTmuxDriver{target: "agy-claude-tmux"})
}
