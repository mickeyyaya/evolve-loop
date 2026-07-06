package bridge

// launchintent.go — the CLI-agnostic launch abstraction (ADR-0022).
//
// A phase agent describes HOW it wants to be launched in high-level terms
// (which model tier, what permission posture, project-only settings, an
// ephemeral or named session). A per-CLI Realizer translates that intent into
// the concrete realization for ONE CLI — launch flags it actually defines,
// post-boot REPL input, controller (tmux lifecycle) hints — never a flag the
// target CLI does not understand. This decouples intent from realization so
// the same intent drives claude, codex, and agy without leaking one CLI's
// argv vocabulary into another.

// LaunchIntent is the high-level, CLI-agnostic launch description. Zero-value
// fields are "unset" and realize to nothing.
type LaunchIntent struct {
	ModelTier     string // abstract tier: haiku | sonnet | opus
	Permission    string // bypass | plan | default
	SettingsScope string // project | all
	SessionMode   string // "ephemeral" | "named:<name>"
	// Effort is the abstract reasoning-effort dial (low | medium | high),
	// realized per-manifest to each CLI's native mechanism (claude --effort,
	// codex model_reasoning_effort) or cleanly no-op'd where the CLI has no
	// such dial (agy/ollama). Empty = unset → emits nothing (purely additive).
	Effort       string
	AllowedTools []string
	// RawByCLI is the per-CLI escape hatch for genuinely CLI-specific argv that
	// has no high-level intent. Flags are applied ONLY to the matching CLI, so
	// a claude-only raw flag never reaches agy/codex.
	RawByCLI map[string][]string
}

// Realization is the concrete, single-CLI materialization of a LaunchIntent.
// The tmux controller (and the headless drivers) consume it: launch with
// LaunchFlags, inject REPLInput after the boot marker, export Env, and honor
// the session-lifecycle hints (Ephemeral / SessionName).
type Realization struct {
	LaunchFlags []string
	REPLInput   []string
	Ephemeral   bool   // controller: kill the session on exit
	SessionName string // controller: named/resumable session ("" = unnamed)
}
