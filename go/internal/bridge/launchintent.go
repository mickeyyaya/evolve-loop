package bridge

// LaunchIntent is the high-level, CLI-agnostic launch description. Zero-value
// fields are "unset" and realize to nothing.
type LaunchIntent struct {
	ModelTier        string // abstract tier: haiku | sonnet | opus
	Permission       string // bypass | plan | default
	SettingsScope    string // project | all
	SessionMode      string // "ephemeral" | "named:<name>"
	Effort           string
	AllowedTools     []string
	SystemPromptFile string
	// RawByCLI is the per-CLI escape hatch for CLI-specific argv with no high-level intent; a claude-only
	// raw flag never reaches agy/codex.
	RawByCLI map[string][]string
}

// Realization is the concrete, single-CLI materialization of a LaunchIntent.
// The tmux controller (and the headless drivers) consume it: launch with
// LaunchFlags, inject REPLInput after the boot marker, export Env, and honor
// the session-lifecycle hints (Ephemeral / SessionName).
type Realization struct {
	LaunchFlags []string
	REPLInput   []string
	Env         map[string]string // the CLI process environment (manifest default_env); exported in a pane, passed to a headless process
	Ephemeral   bool              // controller: kill the session on exit
	SessionName string            // controller: named/resumable session ("" = unnamed)
	// ModelOmitted is the model value the realizer suppressed because it was still an abstract vocabulary
	// token rather than a concrete model id (empty when nothing was suppressed); drivers log it so a launch
	// that fell back to the CLI's own default isn't reported as the requested tier.
	ModelOmitted     string
	SystemPromptFile string
	EffortCapped     string
	EffortVariant    string
	EffortUnapplied  string
	// modelDispatchEffect retains selector, ambiguity and argv-terminator provenance from the final
	// deduplicated LaunchFlags; drivers apply it to their own base selector at the invocation boundary.
	modelDispatchEffect modelDispatch
}
