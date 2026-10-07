package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type walledFirstBridge struct {
	walled string
	clis   []string
}

func (b *walledFirstBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.clis = append(b.clis, req.CLI)
	if req.CLI == b.walled {
		return core.BridgeResponse{ExitCode: 85}, fmt.Errorf("%s: exit=85", req.CLI)
	}
	return core.BridgeResponse{Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}, nil
}

func (*walledFirstBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func routerTableProject(t *testing.T, routerProfile string, block policy.CLIRouting) (string, routerDispatch) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "router.json"), []byte(routerProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	rd, err := resolveRouterDispatchHealthy(profileRouter(t, root, policy.Policy{CLIRouting: &block}), root, decisionPlan, map[string]bool{})
	if err != nil {
		t.Fatalf("resolveRouterDispatchHealthy: %v", err)
	}
	return root, rd
}

func TestRouterAdvisor_WalksTheTablesChainWhenItsFirstCLIIsWalled(t *testing.T) {
	root, rd := routerTableProject(t,
		`{"name":"router","cli":"agy-claude-tmux","model_tier_default":"deep","allowed_clis":["agy-claude","claude"]}`,
		policy.CLIRouting{CLIs: []string{"agy-claude", "claude"}, Default: []string{"agy-claude", "claude"}})
	bridge := &walledFirstBridge{walled: "agy-claude-tmux"}

	plan, err := core.NewPhaseAdvisor(bridge, routerAdvisorOptions(rd)...).Plan(router.RouteInput{Workspace: t.TempDir(), ProjectRoot: root, Cycle: 1})

	if err != nil || plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("a walled agy-claude must hand the advisor to Claude Code, the table's next CLI: %v %+v (tried %v)", err, plan, bridge.clis)
	}
	if got := strings.Join(bridge.clis, ","); got != "agy-claude-tmux,claude-tmux" {
		t.Errorf("CLIs tried %s, want agy-claude-tmux then claude-tmux", got)
	}
}
