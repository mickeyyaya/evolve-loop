package swarmrunner

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestSwarmMerge_SumsWorkerTokens(t *testing.T) {
	sr := swarm.SwarmResult{Workers: []swarm.WorkerResult{
		{WorkerID: "w0", Tokens: cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 5, CacheWrite: 1}},
		{WorkerID: "w1", Tokens: cyclestate.TokenUsage{Input: 300, Output: 40, CacheRead: 15, CacheWrite: 3}},
	}}

	got := sr.TotalTokens()
	want := cyclestate.TokenUsage{Input: 400, Output: 60, CacheRead: 20, CacheWrite: 4}
	if got != want {
		t.Fatalf("TotalTokens = %+v, want %+v", got, want)
	}
}

type tokenBridge struct{ tokens cyclestate.TokenUsage }

func (b tokenBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{ExitCode: 0, Tokens: b.tokens}, nil
}

func (b tokenBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestSwarmMerge_LauncherCarriesTokens(t *testing.T) {
	want := cyclestate.TokenUsage{Input: 42, Output: 7, CacheRead: 2, CacheWrite: 1}
	bl := bridgeLauncher{bridge: tokenBridge{tokens: want}}

	lr, err := bl.Launch(context.Background(), swarm.LaunchRequest{Agent: "build-w0"})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if lr.Tokens != want {
		t.Fatalf("LaunchResult.Tokens = %+v, want %+v", lr.Tokens, want)
	}
}
