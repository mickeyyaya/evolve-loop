package runner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func dispatchedBridgeRequest(t *testing.T, req core.PhaseRequest) core.BridgeRequest {
	t.Helper()
	hooks := &fakeHooks{phase: "build", agent: "evolve-build", model: "sonnet", prompt: "composed", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build report\n"}
	r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-build", "agent body"), VerifyFn: alwaysOKVerify})
	req.ProjectRoot, req.Workspace, req.Worktree = t.TempDir(), t.TempDir(), t.TempDir()
	if _, err := r.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return fb.gotReq
}

func TestRun_AFreshRungLaunchesAnEphemeralSessionWithTheDigest(t *testing.T) {
	try := &retryladder.Try{Rung: retryladder.R2, Context: retryladder.ContextFresh, Stage: retryladder.StageEnforce, Tier: "deep"}
	got := dispatchedBridgeRequest(t, core.PhaseRequest{Ladder: try, CorrectionDirective: "## Retry digest (rung R2, class capability)"})
	if got.Ladder != try {
		t.Fatalf("the bridge request lost the rung: %+v", got.Ladder)
	}
	if got.SessionName != "" || got.FreshSession {
		t.Fatalf("a raised try reuses a session: name %q fresh-retry %v", got.SessionName, got.FreshSession)
	}
	if got.CorrectionDirective != "## Retry digest (rung R2, class capability)" {
		t.Fatalf("the digest did not reach the bridge: %q", got.CorrectionDirective)
	}
}

func TestRun_AnR0DispatchCarriesNoRung(t *testing.T) {
	if got := dispatchedBridgeRequest(t, core.PhaseRequest{}); got.Ladder != nil {
		t.Fatalf("an R0 dispatch carries a rung: %+v", got.Ladder)
	}
}

func TestRouteRequest_TheLadderTierReachesTheRouterOverlay(t *testing.T) {
	b := &BaseRunner{hooks: &fakeHooks{phase: "build", model: "sonnet"}}
	got := b.routeRequest(core.PhaseRequest{Ladder: &retryladder.Try{Tier: "deep"}, ModelRoutingTier: "fast"}, phasePreparation{phase: "build", profileName: "builder"}, nil)
	if got.Overlay.LadderTier != "deep" || got.Overlay.Tier != "fast" {
		t.Fatalf("overlay = %+v, want ladder tier deep beside the advisor tier fast", got.Overlay)
	}
	if plain := b.routeRequest(core.PhaseRequest{}, phasePreparation{phase: "build", profileName: "builder"}, nil); plain.Overlay.LadderTier != "" {
		t.Fatalf("an R0 request set ladder tier %q", plain.Overlay.LadderTier)
	}
}
