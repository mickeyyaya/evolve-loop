package recovery

import "strings"

// TerminalCause is the typed classification of a fatal terminal state
// (ADR-0044 C2). "exit 81" is not a cause; "claude booted into an
// inaccessible-model error" is — recovery decisions are made on typed causes,
// never on raw exit codes or untyped pane text (design principle #2,
// classify-before-you-handle).
type TerminalCause string

const (
	// CauseModelInvalid: the CLI booted into its invalid/inaccessible-model
	// error (cycle-262 retro: `claude --model auto`). The REPL is unusable;
	// no amount of waiting produces an artifact.
	CauseModelInvalid TerminalCause = "model_invalid"
	// CauseCLISelfUpdated: the CLI updated its own binary mid-launch and asked
	// for a restart (cycle-262 build: codex self-upgrade). The REPL exited;
	// the pane is (or is about to become) a bare shell.
	CauseCLISelfUpdated TerminalCause = "cli_self_updated"
	// CauseDeadShell: the pane is a plain shell, not an agent REPL — the
	// telltale is the shell rejecting agent-directed input (the bridge's
	// nudge echoing back as "command not found").
	CauseDeadShell TerminalCause = "dead_shell"
	// CauseUnknown: no seeded signature matched. The deterministic layer
	// classifies nothing here — escalation (the LLM failure-advisor tail,
	// Slice 5) owns novel states.
	CauseUnknown TerminalCause = "unknown"
)

// FatalSignature is one entry in the deterministic fatal-pane registry: a
// pane substring that self-describes an unrecoverable terminal state, and the
// typed cause it maps to. Substring matching (not regex) keeps the hot-loop
// check trivially cheap and the registry trivially promotable — the Slice-5
// LLM advisor promotes novel signatures as plain substrings.
type FatalSignature struct {
	Substr string
	Cause  TerminalCause
	// Note documents provenance (which incident taught us this signature) —
	// carried into the justification trail when the signature matches.
	Note string
}

// FatalPaneDetector is the ordered deterministic registry consulted by the
// bridge's stop-review checkpoint. Classification is side-effect-free and
// always-on; ACTING on a classification (fast-fail instead of burning the
// maxExtends backstop) is the caller's stage-gated decision
// (EVOLVE_PHASE_RECOVERY: off | shadow | enforce).
//
// False-positive posture: a working agent's pane can mention any of these
// strings (an editor showing this very file, a test log). Three independent
// defenses bound the risk: (1) the caller consults the detector only at a
// stop-review checkpoint (the artifact is already missing), (2) the caller
// skips fatal handling while the pane is visibly Busy (a working agent is
// never killed), and (3) the kill action itself is stage-gated, shipping in
// shadow (log-only) first.
type FatalPaneDetector struct {
	sigs []FatalSignature
}

// NewFatalPaneDetector builds a detector over an ordered signature list.
// First match wins — order entries most-specific first.
func NewFatalPaneDetector(sigs []FatalSignature) *FatalPaneDetector {
	return &FatalPaneDetector{sigs: sigs}
}

// SeedDetector returns the registry seeded with the known-fatal signatures,
// all three taught by cycle-262 (2026-06-09; forensics in
// docs/architecture/phase-recovery.md §2). Slice 5 adds durable promotion of
// advisor-classified novel signatures on top of these seeds.
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
			// The colon-prefixed form is the shell's OWN error rendering —
			// zsh emits it mid-line ("zsh: command not found: X"), bash at
			// line end ("bash: X: command not found") — and is far harder to
			// false-positive than the bare phrase, which can appear in a
			// healthy agent's debugging output.
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

// Detect scans a recent pane tail for a seeded fatal signature. It returns
// the typed cause plus the matched substring (for the justification trail),
// or ok=false when nothing matches. Pure classification — no action, no
// side effects, nil-receiver safe (a nil detector classifies nothing).
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
