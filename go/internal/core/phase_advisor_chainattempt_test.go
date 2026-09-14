package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
)

// TestBridgeRequestOf_MarksTheAdvisorsAttempt — the advisor walks its own CLI
// chain (llmroute.Dispatch, cycle-435); each launch it hands the bridge is one
// attempt of that walk and must say so, or a chain-walking bridge handle would
// resolve and walk the chain a second time around it.
func TestBridgeRequestOf_MarksTheAdvisorsAttempt(t *testing.T) {
	req := bridgeRequestOf(advisor.LaunchRequest{CLI: "codex-tmux", Model: "deep", Agent: "router"})
	if !req.ChainAttempt || req.CLI != "codex-tmux" {
		t.Fatalf("advisor attempts must be marked ChainAttempt: %+v", req)
	}
}
