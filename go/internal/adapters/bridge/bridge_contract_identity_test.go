package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaunch_UsesExplicitDeliverableContract(t *testing.T) {
	fe := &fakeEngine{}
	req := core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/profiles/router.json", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/ws/routing-proposal.json", Agent: "router",
		Contract: "router-proposal",
	}

	if _, err := withEngine(fe).Launch(context.Background(), req); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !strings.Contains(fe.gotReq.Prompt, "Deliverable Contract (router-proposal)") {
		t.Errorf("router proposal must receive its own contract; prompt:\n%s", fe.gotReq.Prompt)
	}
	if !strings.Contains(fe.gotReq.Prompt, "evolve phase verify router-proposal") {
		t.Errorf("router proposal must self-check its own artifact; prompt:\n%s", fe.gotReq.Prompt)
	}
}

func TestLaunch_RejectsUnknownExplicitDeliverableContract(t *testing.T) {
	req := core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/profiles/router.json", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/ws/routing-proposal.json", Agent: "router",
		Contract: "missing-router-contract",
	}

	_, err := withEngine(&fakeEngine{}).Launch(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), `deliverable contract "missing-router-contract" not registered`) {
		t.Fatalf("unknown explicit contract must fail with context; got %v", err)
	}
}
