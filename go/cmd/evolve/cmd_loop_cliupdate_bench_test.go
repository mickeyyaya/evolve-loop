package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func agyClaudeHost(smokeFailing ...string) *fakeCLIHost {
	h := &fakeCLIHost{
		families: []cliupdate.Family{
			{Name: "agy", UpdateArgv: []string{"agy", "update"}},
			{Name: "claude", UpdateArgv: []string{"claude", "update"}},
		},
		versions: map[string][]string{"agy": {"1.5.3", "1.5.9"}, "claude": {"2.1.285", "2.1.286"}},
		smokeErr: map[string]error{},
	}
	for _, family := range smokeFailing {
		h.smokeErr[family] = errors.New("doctor live " + family + "-tmux rc=87 pattern=\"\"")
	}
	return h
}

func routerOf(t *testing.T, block policy.CLIRouting) *cliroute.Router {
	t.Helper()
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy:   policy.Policy{CLIRouting: &block},
		Profiles: profiles.NewFromDir(t.TempDir()),
		Host:     cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	return r
}

func bothFamiliesRouted() policy.CLIRouting {
	return policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
}

func TestBootCLIUpdate_AFamilyThatFailsItsSmokeIsBenchedAndTheWaveStarts(t *testing.T) {
	b, _, signalOut := boundaryCoordinator(t, 0)
	b.deps.Router = routerOf(t, bothFamiliesRouted())
	agyClaudeHost("agy").install(t)
	lr := &loopResult{}
	var stdout, stderr bytes.Buffer

	halted := bootCLIUpdateHalts(context.Background(), b.cfg, b.deps, lr, &stdout, &stderr)
	b.deps.Signals.Flush()

	if halted || lr.StopReason != "" || len(lr.CLIUpdateHalt) != 0 || stdout.Len() != 0 {
		t.Fatalf("claude can run every lane, so the agy smoke failure must not halt the loop: halted=%v stop=%q halt=%+v\n%s", halted, lr.StopReason, lr.CLIUpdateHalt, stderr.String())
	}
	active := clihealth.NewStore(b.cfg.ProjectRoot, nil).Active()
	entry, benched := active["agy"]
	if !benched {
		t.Fatalf("the smoke-failed family must be benched in clihealth: %+v", active)
	}
	if _, claudeBenched := active["claude"]; claudeBenched {
		t.Errorf("the passing family must stay unbenched: %+v", active)
	}
	if !strings.Contains(entry.Reason, "smoke") || !entry.BenchedUntil.After(entry.BenchedAt) || !strings.Contains(entry.Evidence, "rc=87") {
		t.Errorf("the bench carries the smoke reason, an expiry and the smoke result as evidence: %+v", entry)
	}
	if strings.Contains(signalOut.String(), "LOOP_HALT") {
		t.Errorf("a benched family is no loop halt:\n%s", signalOut.String())
	}
	if n := strings.Count(signalOut.String(), "INCIDENT"); n != 1 {
		t.Errorf("want exactly one INCIDENT signal, got %d:\n%s", n, signalOut.String())
	}
	if out := signalOut.String(); !strings.Contains(out, "agy") || !strings.Contains(out, "smoke") {
		t.Errorf("the INCIDENT names the family and the failing stage:\n%s", out)
	}
}

func TestBootCLIUpdate_TheRoutesSkipAFamilyBenchedBySmoke(t *testing.T) {
	b, _, _ := boundaryCoordinator(t, 0)
	router := routerOf(t, bothFamiliesRouted())
	b.deps.Router = router
	agyClaudeHost("agy").install(t)

	bootCLIUpdateHalts(context.Background(), b.cfg, b.deps, &loopResult{}, io.Discard, io.Discard)
	b.deps.Signals.Flush()

	rd, err := resolveRouterDispatchHealthy(router, b.cfg.ProjectRoot, decisionPlan, benchedFamilies(b.cfg.ProjectRoot))
	if err != nil {
		t.Fatalf("resolve the route: %v", err)
	}
	if llmroute.Family(rd.cli) != "claude" || !rd.healthy {
		t.Errorf("the route must skip the benched agy and land on claude: cli=%q healthy=%v", rd.cli, rd.healthy)
	}
}

func TestBootCLIUpdate_AFamilyThatNoRouteCanDoWithoutHaltsTheLoop(t *testing.T) {
	cases := map[string]struct {
		block   policy.CLIRouting
		failing []string
	}{
		"every routed family fails": {block: bothFamiliesRouted(), failing: []string{"agy", "claude"}},
		"the floor family claude fails while agy is healthy": {
			block:   bothFamiliesRouted(),
			failing: []string{"claude"},
		},
		"one role pins the failing family": {
			block: policy.CLIRouting{
				CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"},
				Work: map[string][]string{"plan": {"agy"}},
			},
			failing: []string{"agy"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b, _, signalOut := boundaryCoordinator(t, 0)
			b.deps.Router = routerOf(t, tc.block)
			agyClaudeHost(tc.failing...).install(t)
			lr := &loopResult{}
			var stdout, stderr bytes.Buffer

			halted := bootCLIUpdateHalts(context.Background(), b.cfg, b.deps, lr, &stdout, &stderr)
			b.deps.Signals.Flush()

			if !halted || lr.StopReason != cliUpdateSmokeHalt || len(lr.CLIUpdateHalt) == 0 {
				t.Fatalf("a route has no family left to run: the boot must halt as today: halted=%v stop=%q halt=%+v\n%s", halted, lr.StopReason, lr.CLIUpdateHalt, stderr.String())
			}
			if !strings.Contains(signalOut.String(), "LOOP_HALT") || !strings.Contains(stdout.String(), cliUpdateSmokeHalt) {
				t.Errorf("the halt stays a Signal Center incident and a loop result:\n%s\n%s", signalOut.String(), stdout.String())
			}
		})
	}
}

func TestBootCLIUpdate_ASmokeFailureWithNoRoutingTableStillHaltsTheLoop(t *testing.T) {
	b, _, _ := boundaryCoordinator(t, 0)
	agyClaudeHost("agy").install(t)
	lr := &loopResult{}

	halted := bootCLIUpdateHalts(context.Background(), b.cfg, b.deps, lr, io.Discard, io.Discard)

	if !halted || lr.StopReason != cliUpdateSmokeHalt {
		t.Fatalf("with no routing table the loop cannot prove a fallback family: halted=%v stop=%q", halted, lr.StopReason)
	}
	if active := clihealth.NewStore(b.cfg.ProjectRoot, nil).Active(); len(active) != 0 {
		t.Errorf("a halting boot benches nothing: %+v", active)
	}
}

func TestUpdateCLIsAtBoundary_AFamilyThatFailsItsSmokeIsBenchedAndTheNextWaveRuns(t *testing.T) {
	b, _, signalOut := boundaryCoordinator(t, 0)
	b.deps.Router = routerOf(t, bothFamiliesRouted())
	agyClaudeHost("agy").install(t)

	d, stop := b.updateCLIsAtBoundary(1)
	b.deps.Signals.Flush()

	if stop || d.flow != batchProceed || b.result.StopReason != "" {
		t.Fatalf("claude can run every lane: the wave boundary must go on: decision=%+v stop=%v reason=%q", d, stop, b.result.StopReason)
	}
	if _, benched := clihealth.NewStore(b.cfg.ProjectRoot, nil).Active()["agy"]; !benched {
		t.Errorf("the smoke-failed family must be benched at the wave boundary too")
	}
	if n := strings.Count(signalOut.String(), "INCIDENT"); n != 1 || strings.Contains(signalOut.String(), "LOOP_HALT") {
		t.Errorf("want one INCIDENT and no halt:\n%s", signalOut.String())
	}
}

func TestUpdateCLIsAtBoundary_EveryRoutedFamilyFailingItsSmokeHaltsTheNextWave(t *testing.T) {
	b, _, _ := boundaryCoordinator(t, 0)
	b.deps.Router = routerOf(t, bothFamiliesRouted())
	agyClaudeHost("agy", "claude").install(t)

	d, stop := b.updateCLIsAtBoundary(1)
	b.deps.Signals.Flush()

	if !stop || d.flow != batchReturn || d.exitCode != 2 || b.result.StopReason != cliUpdateSmokeHalt {
		t.Fatalf("no routed family can run: the boundary must halt as today: decision=%+v stop=%v reason=%q", d, stop, b.result.StopReason)
	}
}

func TestBootGateHalts_ABenchedSmokeFailureLetsTheReadinessGateRun(t *testing.T) {
	b, _, _ := boundaryCoordinator(t, 0)
	b.deps.Router = routerOf(t, bothFamiliesRouted())
	var order []string
	installPreflightSpy(t, &order)
	agyClaudeHost("agy").install(t)
	lr := &loopResult{}

	halted := bootGateHalts(context.Background(), b.cfg, b.deps, lr, io.Discard, io.Discard)
	b.deps.Signals.Flush()

	if halted || lr.StopReason != "" || len(order) != 1 || order[0] != "preflight" {
		t.Fatalf("the boot path from prepareFreshBatch must reach the readiness gate once agy is benched: halted=%v stop=%q order=%q", halted, lr.StopReason, order)
	}
	if _, benched := clihealth.NewStore(b.cfg.ProjectRoot, nil).Active()["agy"]; !benched {
		t.Errorf("agy must be benched on the production boot path")
	}
}
