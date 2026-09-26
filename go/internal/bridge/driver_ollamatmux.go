package bridge

import (
	"context"
	"fmt"
	"regexp"
)

var ollamaModelTagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]*$`)

func ollamaComposeLaunchCmd(binary, model string, extras []string) string {
	if model == "" {
		model = "llama3.1:8b" // matches manifest default_model
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

// ollamaTmuxDriver drives an interactive `ollama run <model>` REPL through
// tmux — peer of claude-tmux/codex-tmux/agy-tmux. Local vs cloud routing is
// by model tag (a bare tag runs on local hardware, a `:cloud`-suffixed tag
// routes to ollama.com through the same binary and daemon), not an env var
// or driver flag. Plain `ollama run` has no agentic tool use, so Launch
// rejects any source-writing phase rather than run one that cannot succeed.
type ollamaTmuxDriver struct{}

func (ollamaTmuxDriver) Name() string { return "ollama-tmux" }

func (ollamaTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	// cfg.Worktree is non-empty only when the orchestrator marks this phase a
	// writer (core/worktree.go:WorktreePhase, or PhaseSpec.WritesSource).
	if cfg.Worktree != "" {
		fmt.Fprintf(deps.Stderr, "[ollama-tmux] cannot run source-writing phase %q: ollama-tmux has no tool use (no Bash/Edit/Write). Pick claude-tmux / codex-tmux / agy-tmux for writers.\n", cfg.Agent)
		return ExitBadFlags, fmt.Errorf("ollama-tmux: source-writing phase %q rejected (worktree=%s)", cfg.Agent, cfg.Worktree)
	}
	model := cfg.Model
	if model == "" {
		model = "llama3.1:8b" // matches manifest default_model
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
		name:          "ollama-tmux",
		session:       session,
		named:         named,
		launchCmd:     launchCmd,
		modelDispatch: modelDispatch{model: model, source: modelDispatchPositional},
		promptMarker:  ">>> ", // ollama's REPL prompt marker (docs.ollama.com/cli)
		// Same string: the boot-ready marker IS the input-line prompt.
		inputLineMarker: ">>> ",
		bootScrollback:  200, // first-use `ollama run` pulls the model, scrolling the prompt off-screen behind a progress bar
		bootIntervalS:   1,
		tickDuringBoot:  false, // no boot-time interactive prompts on the happy path
		exitSeq:         []tmuxKey{{keys: "/bye", enter: true, pauseS: 1}},
		bootOnly:        cfg.BootOnly,
		guardDeadShell:  true,
	})
}

func init() { Register(ollamaTmuxDriver{}) }
