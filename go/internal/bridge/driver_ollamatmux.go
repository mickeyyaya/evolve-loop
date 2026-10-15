package bridge

import (
	"context"
	"fmt"
	"regexp"
)

var ollamaModelTagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]*$`)

func ollamaComposeLaunchCmd(binary, model string, extras []string) string {
	if model == "" {
		model = "llama3.1:8b"
	}
	out := binary + " run " + model
	for _, x := range extras {
		if x == "" {
			continue
		}
		out += " " + x
	}
	return out
}

type ollamaTmuxDriver struct{}

func (ollamaTmuxDriver) Name() string { return "ollama-tmux" }

func (d ollamaTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	if cfg.Worktree != "" && !HasToolUse(d.Name()) {
		fmt.Fprintf(deps.Stderr, "[ollama-tmux] cannot run worktree phase %q: ollama-tmux has no tool use (no Bash/Edit/Write). Pick claude-tmux / codex-tmux / agy-tmux.\n", cfg.Agent)
		return ExitBadFlags, fmt.Errorf("ollama-tmux: worktree phase %q rejected: no tool use (worktree=%s)", cfg.Agent, cfg.Worktree)
	}
	model := cfg.Model
	if model == "" {
		model = "llama3.1:8b"
	}
	if !ollamaModelTagRe.MatchString(model) {
		fmt.Fprintf(deps.Stderr, "[ollama-tmux] refusing to launch: invalid model tag %q (must match [A-Za-z0-9._:/@-], starting with alnum). Shell-special characters are rejected to prevent send-keys injection.\n", model)
		return ExitBadFlags, fmt.Errorf("ollama-tmux: invalid model tag %q", model)
	}
	if rc, handled := tmuxNonClaudePreflight("ollama-tmux", cfg, deps); handled {
		return rc, nil
	}

	session, named := resolveSession(cfg, deps, "evolve-bridge-ollama-")

	launchCmd := ollamaComposeLaunchCmd(resolveBinary(deps, "ollama"), model, cfg.Realization.LaunchFlags)

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:            "ollama-tmux",
		session:         session,
		named:           named,
		launchCmd:       launchCmd,
		modelDispatch:   modelDispatch{model: model, source: modelDispatchPositional},
		promptMarker:    ">>> ",
		inputLineMarker: ">>> ",
		bootScrollback:  200,
		bootIntervalS:   1,
		tickDuringBoot:  false,
		exitSeq:         []tmuxKey{{keys: "/bye", enter: true, pauseS: 1}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	})
}

func init() { Register(ollamaTmuxDriver{}) }
