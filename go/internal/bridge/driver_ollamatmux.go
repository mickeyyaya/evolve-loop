package bridge

import (
	"context"
	"fmt"
	"regexp"
)

// ollamaModelTagRe constrains the characters a model tag can contain BEFORE it
// is spliced into the `ollama run <model>` launch string. Tmux SendKeys
// delivers the line to a real shell, NOT exec(2), so an unvalidated tag like
// `llama3.1:8b; rm -rf /` would execute the trailing command. The character
// class is the documented ollama tag grammar: `name[:tag][@digest]` where each
// segment is [a-zA-Z0-9._-]+ with optional slashes for namespace tags. Reject
// anything else with a loud error rather than try to shell-escape.
var ollamaModelTagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]*$`)

// ollamaComposeLaunchCmd composes the single REPL launch line `<binary> run
// <model>`. Lives on the driver (not in a test mirror) so test pins exercise
// the SAME function the driver runs — preventing the silent drift the
// reviewer caught when the helper had been duplicated in a test file. The
// binary is parameterized to keep the function pure + trivially testable.
func ollamaComposeLaunchCmd(binary, model string) string {
	if model == "" {
		model = "llama3.1:8b" // matches manifest default_model
	}
	return binary + " run " + model
}

// ollamaTmuxDriver drives an interactive `ollama run <model>` REPL through
// tmux — peer of claude-tmux/codex-tmux/agy-tmux. The user-facing point of
// WS-F: a free local LLM driver for reasoning/review phases (the per-phase
// review gate WS-E will wire as the default reviewer).
//
// Local vs cloud routing is by MODEL TAG, not by env-var or driver flag:
//   - bare tag (`llama3.1:8b`, `qwen2.5-coder:32b`, ...) → local hardware
//   - `:cloud`-suffixed tag (`gpt-oss:120b-cloud`)        → ollama.com hosted
//
// Auth: cloud tags need `OLLAMA_API_KEY` or a prior `ollama signin` (OAuth
// → `~/.ollama/id_ed25519`); local tags need no auth. The same binary +
// daemon serves both — routing happens inside `ollama serve` when a
// `:cloud`-tagged model is requested. `OLLAMA_HOST` picks the daemon
// (default 127.0.0.1:11434), NOT cloud routing.
//
// **Reasoning/review only.** Plain `ollama run` has no agentic tool use
// (no Bash/Edit/Write — text-in/text-out), so a source-writing phase
// (cfg.Worktree != "") is rejected at Launch with a loud error rather than
// run a phase that can't possibly succeed. Tdd/build must stay on
// claude-tmux/codex-tmux/agy-tmux. See the WS-F design constraint in
// `~/.claude/plans/lexical-booping-hamster.md`.
type ollamaTmuxDriver struct{}

// Name implements Driver.
func (ollamaTmuxDriver) Name() string { return "ollama-tmux" }

// Launch starts an `ollama run <model>` tmux REPL session and waits for the
// `>>> ` prompt marker to confirm REPL boot. Exits cleanly via `/bye`.
func (ollamaTmuxDriver) Launch(ctx context.Context, cfg *Config, deps Deps) (int, error) {
	// Reasoning/review only — fail loud on a source-writing assignment.
	// cfg.Worktree is non-empty only when the orchestrator designated this
	// phase as a writer (WorktreePhase: tdd/build, or PhaseSpec.WritesSource
	// for user-defined phases — see core/worktree.go:WorktreePhase).
	if cfg.Worktree != "" {
		fmt.Fprintf(deps.Stderr, "[ollama-tmux] cannot run source-writing phase %q: ollama-tmux has no tool use (no Bash/Edit/Write). Pick claude-tmux / codex-tmux / agy-tmux for writers.\n", cfg.Agent)
		return ExitBadFlags, fmt.Errorf("ollama-tmux: source-writing phase %q rejected (worktree=%s)", cfg.Agent, cfg.Worktree)
	}
	// Strict model-tag validation — input validation MUST precede the safety
	// gate (a malicious model tag would otherwise be silently accepted on a
	// host that already passed AllowBypass). The composed launchCmd reaches
	// the shell via tmux send-keys (NOT exec), so an unvalidated tag is a
	// direct shell-injection vector. See the ollamaModelTagRe comment.
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

	// Model is the POSITIONAL first arg for `ollama run <model>` — NOT a
	// flag. The manifest declares model_tier channel:noop so the realizer
	// emits no model flag; we compose the launch line directly via the
	// shared ollamaComposeLaunchCmd (also exercised by the launch-cmd test
	// pins, so the driver and tests can't silently drift).
	launchCmd := ollamaComposeLaunchCmd(resolveBinary(deps, "ollama"), model)

	return runTmuxREPL(ctx, cfg, deps, tmuxLaunch{
		name:           "ollama-tmux",
		session:        session,
		named:          named,
		launchCmd:      launchCmd,
		promptMarker:   ">>> ", // REPL prompt marker per docs.ollama.com/cli
		bootScrollback: 200,    // first-use `ollama run` prints a model-download progress bar that scrolls the >>> prompt off an 80-row pane; scan scrollback to be safe (operator may also pre-pull via `ollama pull <model>`)
		bootIntervalS:  1,
		tickDuringBoot: false, // no boot-time interactive prompts on the happy path
		exitSeq:        []tmuxKey{{keys: "/bye", enter: true, pauseS: 1}},
	})
}

func init() { Register(ollamaTmuxDriver{}) }
