// Package inbox is the file-based message channel for live command injection
// into a running *-tmux REPL agent.
package inbox

import "path/filepath"

type Kind string

const (
	KindCommand    Kind = "command"
	KindInterrupt  Kind = "interrupt"
	KindNudge      Kind = "nudge"
	KindSystemRule Kind = "system_rule"
	KindKeystroke  Kind = "keystroke"
)

func (k Kind) Valid() bool {
	switch k {
	case KindCommand, KindInterrupt, KindNudge, KindSystemRule, KindKeystroke:
		return true
	default:
		return false
	}
}

type Envelope struct {
	Seq    int64  `json:"seq,omitempty"`
	TS     string `json:"ts"`
	Kind   Kind   `json:"kind"`
	Body   string `json:"body"`
	Source string `json:"source,omitempty"`

	CorrID string `json:"corr_id,omitempty"`

	DeferCount int `json:"defer_count,omitempty"`
}

const dirName = ".bridge-inbox"

func Path(workspace, agent string) string {
	if agent == "" {
		agent = "agent"
	}
	return filepath.Join(workspace, dirName, agent+".ndjson")
}
