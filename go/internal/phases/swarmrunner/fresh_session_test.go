package swarmrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type deadShellFirstBridge struct{ sent []core.BridgeRequest }

func (b *deadShellFirstBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.sent = append(b.sent, req)
	if len(b.sent) == 1 {
		return core.BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}, errors.New("bridge: launch exit=81")
	}
	return core.BridgeResponse{CostUSD: 0.02}, nil
}

func (b *deadShellFirstBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestBridgeLauncher_AWorkerSurvivesADeadShellFirstSession(t *testing.T) {
	b := &deadShellFirstBridge{}

	res, err := bridgeLauncher{bridge: b}.Launch(context.Background(), swarm.LaunchRequest{CLI: "claude-tmux", Agent: "builder-w0", Workspace: t.TempDir()})

	if err != nil || res.ExitCode != 0 || res.CostUSD != 0.02 {
		t.Fatalf("result = %+v, %v; want the fresh session's success", res, err)
	}
	if len(b.sent) != 2 || !b.sent[1].FreshSession || b.sent[1].CLI != "claude-tmux" {
		t.Fatalf("launches = %+v, want one fresh session of the same CLI", b.sent)
	}
}
