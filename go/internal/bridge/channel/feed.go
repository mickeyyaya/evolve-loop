// Package channel is the bidirectional communication channel for long-running
// tmux-REPL phases (ADR-0037): a live filtered inbound feed produced beside the
// observer, plus correlated outbound asks over the ADR-0023 inbox.
package channel

import "path/filepath"

// FeedPath is the canonical per-agent live feed file. Producer (sole writer),
// Supervisor, and `evolve bridge watch` MUST all call this so they agree on the
// path. An empty agent defaults to "agent", mirroring inbox.Path.
func FeedPath(workspace, agent string) string {
	if agent == "" {
		agent = "agent"
	}
	return filepath.Join(workspace, agent+"-channel.ndjson")
}
