package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
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

// prepareTmuxREPL finishes before any tmux side effect.
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

	prep.namedExists = reportNamedSession(ctx, deps, lp, prep.prefix)
	prep.resolvedPromptFile = filepath.Join(cfg.Workspace, "resolved-prompt.txt")
	facts := identityFacts(cfg, lp.session, prep.resolvedPromptFile)
	authority, err := stateAuthority(cfg.Realization.SystemPromptFile)
	if err != nil {
		return prep, ExitBadFlags, fmt.Errorf("%s write system prompt: %w", prep.prefix, err)
	}
	if !lp.bootOnly {
		prompt, err := preparePrompt(cfg, deps)
		if err != nil {
			return prep, ExitBadFlags, err
		}
		prep.resolvedPrompt = withIdentity(prompt, authority, facts)
		if err := os.WriteFile(prep.resolvedPromptFile, []byte(prep.resolvedPrompt+"\n"), 0o644); err != nil {
			return prep, ExitBadFlags, fmt.Errorf("%s write resolved prompt: %w", prep.prefix, err)
		}
	}
	if prep.namedExists && sandboxRequiredButUnavailable(deps, cfg, false) {
		fmt.Fprintf(deps.Stderr, "%s safety gate: existing named session cannot prove the requested sandbox policy; launch a new session\n", prep.prefix)
		return prep, ExitSafetyGate, nil
	}

	prep.scrollbackFile = filepath.Join(cfg.Workspace, "tmux-final-scrollback.txt")
	prep.artifactScrollback = defaultIfZero(deps.ScrollbackLines, tmuxArtifactScrollback)
	reportLaunchModel(deps, cfg, prep.prefix, lp.session, prep.workingDir)
	return prep, ExitOK, nil
}

func reportLaunchModel(deps Deps, cfg *Config, prefix, session, workingDir string) {
	if omitted := cfg.Realization.ModelOmitted; omitted != "" {
		fmt.Fprintf(deps.Stderr, "%s model='%s' is an unresolved tier token → no --model sent; this pane runs the CLI's OWN default, not the requested tier\n", prefix, omitted)
	}
	if capped := cfg.Realization.EffortCapped; capped != "" {
		fmt.Fprintf(deps.Stderr, "%s effort=%s is capped to the %s model variant: the CLI offers no higher effort for this model\n", prefix, capped, cfg.Realization.EffortVariant)
	}
	if unapplied := cfg.Realization.EffortUnapplied; unapplied != "" {
		fmt.Fprintf(deps.Stderr, "%s WARN effort=%s is not applied: the model name has no (Variant) suffix\n", prefix, unapplied)
	}
	fmt.Fprintf(deps.Stderr, "%s session=%s model=%s workdir=%s\n", prefix, session,
		orDefault(effectiveModelLabel(cfg.Model, cfg.Realization.ModelOmitted), "(cli default)"), workingDir)
}

func reportNamedSession(ctx context.Context, deps Deps, lp tmuxLaunch, prefix string) bool {
	if !lp.named {
		return false
	}
	exists := deps.Tmux.HasSession(ctx, lp.session)
	if exists {
		fmt.Fprintf(deps.Stderr, "%s RESUME: reattaching to existing named session '%s'\n", prefix, lp.session)
	} else {
		fmt.Fprintf(deps.Stderr, "%s CREATE-NAMED: new named session '%s' (persists on exit for resume)\n", prefix, lp.session)
	}
	return exists
}

func identityFacts(cfg *Config, session, pastedFile string) string {
	return phaseidentity.Block(phaseidentity.Facts{
		Agent: cfg.Agent, Cycle: cfg.Cycle, Session: session,
		PromptFile: cfg.PromptFile, PastedFile: pastedFile, Artifact: writtenArtifact(cfg),
	})
}

func stateAuthority(systemPromptFile string) (string, error) {
	if systemPromptFile == "" {
		return phaseidentity.Authority(), nil
	}
	return "", atomicwrite.Bytes(systemPromptFile, []byte(phaseidentity.Authority()))
}

func withIdentity(prompt, authority, facts string) string {
	if facts == "" {
		return prompt
	}
	blocks := strings.TrimRight(facts, "\n")
	if authority != "" {
		blocks = strings.TrimRight(authority, "\n") + "\n\n" + blocks
	}
	return strings.TrimRight(prompt, "\n") + "\n\n" + blocks
}

func writtenArtifact(cfg *Config) string {
	if cfg.Completion == completionStdout {
		return ""
	}
	return cfg.Artifact
}
