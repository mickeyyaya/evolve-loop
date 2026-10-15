// Package channel is the bidirectional communication channel for long-running
// tmux-REPL phases: a live filtered inbound feed plus correlated outbound asks.
// See docs/architecture/packages/internal-bridge-channel.md.
package channel

import "path/filepath"

func FeedPath(workspace, agent string) string {
	if agent == "" {
		agent = "agent"
	}
	return filepath.Join(workspace, agent+"-channel.ndjson")
}
