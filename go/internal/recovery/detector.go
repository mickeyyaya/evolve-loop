package recovery

import "strings"

// TerminalCause is the typed classification of a fatal terminal state.
type TerminalCause string

const (
	// CauseModelInvalid means the CLI booted into its invalid-model error.
	CauseModelInvalid TerminalCause = "model_invalid"
	// CauseCLISelfUpdated means the CLI updated its own binary and exited.
	CauseCLISelfUpdated TerminalCause = "cli_self_updated"
	// CauseDeadShell means the pane is a plain shell, not an agent REPL.
	CauseDeadShell TerminalCause = "dead_shell"
	// CauseUnknown means no signature matched; the LLM failure-advisor owns it.
	CauseUnknown TerminalCause = "unknown"
)

// SessionRecoverable reports whether one fresh session of the same CLI can
// succeed: the REPL is gone while the CLI and account are fine. A model or
// config cause fails the same way again, so the chain must move on.
func (c TerminalCause) SessionRecoverable() bool {
	return c == CauseDeadShell || c == CauseCLISelfUpdated
}

// FatalSignature maps a pane substring to the terminal cause it identifies.
type FatalSignature struct {
	Substr string
	Cause  TerminalCause
	// Note records provenance and is carried into the justification trail.
	Note string
}

// FatalPaneDetector is the ordered, first-match-wins registry of fatal signatures.
type FatalPaneDetector struct {
	sigs []FatalSignature
}

// NewFatalPaneDetector builds a detector; order sigs most specific first, since the first match wins.
func NewFatalPaneDetector(sigs []FatalSignature) *FatalPaneDetector {
	return &FatalPaneDetector{sigs: sigs}
}

// SeedDetector returns the registry seeded with the known fatal signatures.
func SeedDetector() *FatalPaneDetector {
	return NewFatalPaneDetector([]FatalSignature{
		{
			Substr: "There's an issue with the selected model",
			Cause:  CauseModelInvalid,
			Note:   "claude boot error (cycle-262 retro: --model auto)",
		},
		{
			Substr: "Update ran successfully! Please restart",
			Cause:  CauseCLISelfUpdated,
			Note:   "codex self-upgrade mid-launch (cycle-262 build)",
		},
		{
			// The colon-prefixed form is the shell's own error rendering; the bare
			// phrase can appear in a healthy agent's debugging output.
			Substr: ": command not found",
			Cause:  CauseDeadShell,
			Note:   "shell rejecting agent-directed input — the REPL is gone (cycle-262: nudged bare zsh)",
		},
		{
			Substr: "\nquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh continuation prompt after prompt spill — the REPL is gone (cycle-274 codex update-menu wedge)",
		},
		{
			Substr: "\nbquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh backquote continuation prompt after prompt spill — the REPL is gone (cycle-274 codex update-menu wedge)",
		},
		{
			Substr: "\ndquote>",
			Cause:  CauseDeadShell,
			Note:   "zsh double-quote continuation prompt after prompt spill — the REPL is gone (cycle-274/277 transcript variant, R3.3)",
		},
		{
			Substr: "\nheredoc>",
			Cause:  CauseDeadShell,
			Note:   "zsh heredoc continuation prompt after prompt spill — the REPL is gone (cycle-274/277 transcript variant, R3.3)",
		},
	})
}

// Signatures reports the live registry's non-empty substrings in order, promotions included; nil-receiver safe.
func (d *FatalPaneDetector) Signatures() []string {
	if d == nil {
		return nil
	}
	out := make([]string, 0, len(d.sigs))
	for _, sig := range d.sigs {
		if sig.Substr != "" {
			out = append(out, sig.Substr)
		}
	}
	return out
}

// Detect returns the cause and substring of the first signature found in pane; nil-receiver safe.
func (d *FatalPaneDetector) Detect(pane string) (TerminalCause, string, bool) {
	if d == nil || pane == "" {
		return CauseUnknown, "", false
	}
	for _, sig := range d.sigs {
		if sig.Substr != "" && strings.Contains(pane, sig.Substr) {
			return sig.Cause, sig.Substr, true
		}
	}
	return CauseUnknown, "", false
}
