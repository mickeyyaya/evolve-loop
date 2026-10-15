package bridge

import (
	"context"
	"fmt"
)

type codexTmuxDriver struct{}

func (codexTmuxDriver) Name() string { return "codex-tmux" }

func (codexTmuxDriver) Preflight(ctx context.Context, cfg *Config, deps Deps) error {
	_ = ctx
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
	if v, ok := lookupEnv(deps, "OPENAI_API_KEY"); ok && v != "" {
		if allow, _ := lookupEnv(deps, "BRIDGE_ALLOW_OPENAI_API_KEY"); allow != "1" {
			fmt.Fprintln(deps.Stderr, "[codex-tmux] credential-isolation guard: OPENAI_API_KEY set without BRIDGE_ALLOW_OPENAI_API_KEY=1")
			return ExitCostLeak, nil
		}
	}

	session, named := resolveSession(cfg, deps, "evolve-bridge-codex-")

	flags := cfg.Realization.LaunchFlags
	dispatched := modelDispatchFromRealization("codex-tmux", defaultModelDispatch(), cfg.Realization)
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
		name:            "codex-tmux",
		session:         session,
		named:           named,
		launchCmd:       launchCmd,
		modelDispatch:   dispatched,
		promptMarker:    "›",
		inputLineMarker: "›",
		bootScrollback:  200,
		bootIntervalS:   2,
		tickDuringBoot:  true,
		exitSeq:         []tmuxKey{{keys: "/quit", enter: true, pauseS: 2}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	})
}

func init() { Register(codexTmuxDriver{}) }
