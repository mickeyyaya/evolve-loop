package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
)

func TestBridgeRequestOf_MarksTheAdvisorsAttempt(t *testing.T) {
	req := bridgeRequestOf(advisor.LaunchRequest{CLI: "codex-tmux", Model: "deep", Agent: "router"})
	if !req.ChainAttempt || req.CLI != "codex-tmux" {
		t.Fatalf("advisor attempts must be marked ChainAttempt: %+v", req)
	}
}
