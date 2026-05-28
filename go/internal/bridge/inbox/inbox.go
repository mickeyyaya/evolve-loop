// Package inbox is the file-based message channel for live command
// injection into a running *-tmux REPL agent. A sender (the `evolve bridge
// send` CLI, or the phase-observer's nudge hook) appends NDJSON envelopes
// to <workspace>/.bridge-inbox/<agent>.ndjson; the tmux driver drains them
// from its existing artifact-wait poll loop and injects them into the live
// session.
//
// It is a leaf package (no internal deps) so the bridge driver, the CLI
// command, and the phase-observer can all import it without a cycle.
package inbox

import "path/filepath"

// Kind enumerates the injection semantics of an envelope.
//
//	command     — inject when the agent is idle (prompt marker visible).
//	interrupt   — send ESC first, then inject regardless of agent state.
//	nudge       — observer-originated command; idle-gated like command.
//	system_rule — late rule injection; idle-gated, prefixed "## Rules".
//	keystroke   — raw tmux send-keys, no idle-gate, no ESC prefix, no Enter
//	              suffix. Body is one tmux key-spec (literal text or a named
//	              key like "Enter" / "Escape" / "C-c" / "Up", or several
//	              space-separated tokens) sent verbatim. This is the operator's
//	              "full tmux control" hatch (cycle-124 F4 / ADR-0023): used by
//	              `evolve bridge send` and the phase-observer to dismiss
//	              modals, navigate menus, and confirm y/N prompts that no
//	              other Kind handles. The operator carries full responsibility
//	              for what reaches the running REPL.
type Kind string

const (
	KindCommand    Kind = "command"
	KindInterrupt  Kind = "interrupt"
	KindNudge      Kind = "nudge"
	KindSystemRule Kind = "system_rule"
	KindKeystroke  Kind = "keystroke"
)

// Valid reports whether k is a recognized envelope kind.
func (k Kind) Valid() bool {
	switch k {
	case KindCommand, KindInterrupt, KindNudge, KindSystemRule, KindKeystroke:
		return true
	default:
		return false
	}
}

// Envelope is one NDJSON line in an agent inbox.
type Envelope struct {
	Seq    int64  `json:"seq,omitempty"`    // best-effort writer hint; the reader's cursor is authoritative
	TS     string `json:"ts"`               // RFC3339 UTC mint time
	Kind   Kind   `json:"kind"`             // injection semantics
	Body   string `json:"body"`             // text to inject
	Source string `json:"source,omitempty"` // "cli" | "observer" | custom

	// DeferCount tracks how many times a mid-turn command was re-queued by
	// the driver while the agent was busy. Bounded so a never-idle agent
	// cannot loop the inbox forever.
	DeferCount int `json:"defer_count,omitempty"`
}

// dirName is the inbox subdirectory under a workspace.
const dirName = ".bridge-inbox"

// Path derives the canonical inbox file path for an agent. The sender and
// the draining driver MUST both call this so they compute identical paths.
// An empty agent defaults to "agent", mirroring engine.go's prompt-file
// convention.
func Path(workspace, agent string) string {
	if agent == "" {
		agent = "agent"
	}
	return filepath.Join(workspace, dirName, agent+".ndjson")
}
