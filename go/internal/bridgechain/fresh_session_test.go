package bridgechain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type deadShellOnce struct{ calls []core.BridgeRequest }

func (d *deadShellOnce) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	d.calls = append(d.calls, req)
	if len(d.calls) == 1 {
		return core.BridgeResponse{ExitCode: 81, CauseCode: "dead_shell", SessionRecoverable: true}, errors.New("bridge: exit 81")
	}
	return core.BridgeResponse{}, nil
}

func (d *deadShellOnce) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func TestWalking_ADeadShellGetsOneFreshSessionOfTheSameCLIBeforeTheWalkMovesOn(t *testing.T) {
	inner := &deadShellOnce{}
	w := bridgechain.New(inner, fixedPlan([]string{"codex-tmux", "claude-tmux"}, []string{"deep"}))

	resp, err := w.Launch(context.Background(), core.BridgeRequest{CLI: "codex-tmux", Model: "deep", Agent: "retrospective"})

	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("resp = %+v err = %v, want the fresh session's success", resp, err)
	}
	if got := strings.Join(attempts(inner.calls), " "); got != "codex-tmux@deep codex-tmux@deep" || !inner.calls[1].FreshSession {
		t.Fatalf("attempts = %s, want one fresh session of codex-tmux before any walk", got)
	}
}
