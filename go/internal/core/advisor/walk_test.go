package advisor

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestLaunch_WalksTheRouteItIsGivenPastAWalledFirstCLI(t *testing.T) {
	fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-claude-tmux", 85), okPlan()}}
	id := Identity{CLI: "agy-claude-tmux", Model: "deep", AgentLabel: "router"}
	route := WithRoute(llmroute.Plan{Candidates: []string{"agy-claude-tmux", "claude-tmux"}, Triggers: []int{85}})

	plan, err := New(fl, id, plainWriter, route).Plan(tempInput(t))

	if err != nil || plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("the second CLI of the route answers after the first is walled: %v %+v", err, plan)
	}
	if got := strings.Join(fl.calledCLIs(), ","); got != "agy-claude-tmux,claude-tmux" {
		t.Errorf("CLIs tried %s, want the route in order", got)
	}
}

func TestLaunch_WithNoRouteDispatchesTheIdentityCLIAloneAndNeverReadsAProfileChain(t *testing.T) {
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), okPlan()}}
	in := tempInput(t)
	in.ProjectRoot = root

	_, err := New(fl, Identity{CLI: "agy-tmux", Model: "deep", AgentLabel: "router"}, plainWriter).Plan(in)

	if err == nil || fl.calls != 1 {
		t.Fatalf("without a route the advisor launches its CLI once; the profile's cli_fallback is the router's business: %v (%d calls)", err, fl.calls)
	}
}
