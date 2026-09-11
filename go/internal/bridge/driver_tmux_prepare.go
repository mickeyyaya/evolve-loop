package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type replPreparation struct {
	prefix             string
	workingDir         string
	resolvedPromptFile string
	resolvedPrompt     string
	namedExists        bool
	scrollbackFile     string
	artifactScrollback int
}

// prepareTmuxREPL validates launch inputs and materializes the immutable files
// and identities used by the session. It finishes before any tmux side effect.
func prepareTmuxREPL(ctx context.Context, cfg *Config, deps Deps, lp tmuxLaunch) (replPreparation, int, error) {
	prep := replPreparation{prefix: "[" + lp.name + "]"}

	prep.workingDir = cfg.Worktree
	if prep.workingDir == "" {
		if v, _ := lookupEnv(deps, ipcenv.FleetKey); envchain.BoolValue(v, false) {
			return prep, ExitBadFlags, fmt.Errorf("%s %w", prep.prefix, errWorktreeRequired)
		}
		prep.workingDir, _ = os.Getwd()
		fmt.Fprintf(deps.Stderr, "%s WARN no worktree designated — falling back to process cwd %s (single-driver mode only; fleet mode refuses this)\n", prep.prefix, prep.workingDir)
	}
	if !IsDir(prep.workingDir) {
		fmt.Fprintf(deps.Stderr, "%s working dir does not exist: %s\n", prep.prefix, prep.workingDir)
		return prep, ExitBadFlags, nil
	}
	if err := ensureDirs(cfg); err != nil {
		return prep, ExitBadFlags, fmt.Errorf("%s %w", prep.prefix, err)
	}

	if !lp.bootOnly {
		prompt, err := preparePrompt(cfg, deps)
		if err != nil {
			return prep, ExitBadFlags, err
		}
		prep.resolvedPrompt = prompt
		prep.resolvedPromptFile = filepath.Join(cfg.Workspace, "resolved-prompt.txt")
		if err := os.WriteFile(prep.resolvedPromptFile, []byte(prompt+"\n"), 0o644); err != nil {
			return prep, ExitBadFlags, fmt.Errorf("%s write resolved prompt: %w", prep.prefix, err)
		}
	}

	if lp.named {
		prep.namedExists = deps.Tmux.HasSession(ctx, lp.session)
		if prep.namedExists {
			fmt.Fprintf(deps.Stderr, "%s RESUME: reattaching to existing named session '%s'\n", prep.prefix, lp.session)
		} else {
			fmt.Fprintf(deps.Stderr, "%s CREATE-NAMED: new named session '%s' (persists on exit for resume)\n", prep.prefix, lp.session)
		}
	}
	if prep.namedExists && sandboxRequiredButUnavailable(deps, cfg, false) {
		fmt.Fprintf(deps.Stderr, "%s safety gate: existing named session cannot prove the requested sandbox policy; launch a new session\n", prep.prefix)
		return prep, ExitSafetyGate, nil
	}

	prep.scrollbackFile = filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")
	prep.artifactScrollback = defaultIfZero(deps.ScrollbackLines, tmuxArtifactScrollback)
	if omitted := cfg.Realization.ModelOmitted; omitted != "" {
		fmt.Fprintf(deps.Stderr, "%s model='%s' is an unresolved tier token → no --model sent; this pane runs the CLI's OWN default, not the requested tier\n", prep.prefix, omitted)
	}
	fmt.Fprintf(deps.Stderr, "%s session=%s model=%s workdir=%s\n", prep.prefix, lp.session,
		orDefault(effectiveModelLabel(cfg.Model, cfg.Realization.ModelOmitted), "(cli default)"), prep.workingDir)
	return prep, ExitOK, nil
}
