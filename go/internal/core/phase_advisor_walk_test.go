package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

type walledBridge struct {
	walled string
	clis   []string
}

func (b *walledBridge) Launch(_ context.Context, req BridgeRequest) (BridgeResponse, error) {
	b.clis = append(b.clis, req.CLI)
	if req.CLI == b.walled {
		return BridgeResponse{ExitCode: 85}, fmt.Errorf("%s: exit=85", req.CLI)
	}
	return BridgeResponse{Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}, nil
}

func (*walledBridge) Probe(context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

func TestWithProposerWalk_TheAdvisorWalksTheRouteItIsGiven(t *testing.T) {
	walk := llmroute.Plan{Candidates: []string{"agy-claude-tmux", "claude-tmux"}, Triggers: []int{85}}
	for name, tc := range map[string]struct {
		opts []PhaseAdvisorOption
		want string
	}{
		"with the walk":    {[]PhaseAdvisorOption{WithProposerCLI("agy-claude-tmux"), WithProposerWalk(walk)}, "agy-claude-tmux,claude-tmux"},
		"without the walk": {[]PhaseAdvisorOption{WithProposerCLI("agy-claude-tmux")}, "agy-claude-tmux"},
	} {
		t.Run(name, func(t *testing.T) {
			b := &walledBridge{walled: "agy-claude-tmux"}
			in := baseRouteInput()
			in.Workspace = t.TempDir()
			_, _ = NewPhaseAdvisor(b, tc.opts...).Plan(in)
			if got := strings.Join(b.clis, ","); got != tc.want {
				t.Fatalf("CLIs tried %s, want %s", got, tc.want)
			}
		})
	}
}
