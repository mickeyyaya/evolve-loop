//go:build spike

package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/cmd/evolve/cmdutil"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestSpikeAdvisorLive(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	cfg, _ := config.Load(registry, filterEvolveEnv(os.Environ()))

	br := bridge.NewDefault(root, nil)
	cliRouter, _, err := loadCLIRouter(root, routingHost(io.Discard, time.Now))
	if err != nil {
		t.Fatalf("routing table: %v", err)
	}
	rd, err := resolveRouterDispatchHealthy(cliRouter, root, decisionPlan, benchedFamilies(root))
	if err != nil {
		t.Fatalf("router dispatch: %v", err)
	}
	cli, model := rd.cli, rd.model
	var persona string
	if rp, perr := cmdutil.NewPromptsLoader(root).Agent("evolve-router"); perr == nil {
		persona = rp.Body
	}
	t.Logf("SPIKE: advisor cli=%s model=%s persona=%dB", cli, model, len(persona))
	adv := core.NewPhaseAdvisor(br,
		core.WithProposerCLI(cli), core.WithProposerModel(model), core.WithPersona(persona))

	in := router.RouteInput{
		Current:     string(core.PhaseStart),
		Cfg:         cfg,
		Workspace:   ws,
		ProjectRoot: root,
		Cycle:       999,
		Env:         filterEvolveEnv(os.Environ()),
		Catalog: []router.PhaseCard{
			{Name: "scout", Role: "plan"},
			{Name: "tdd", Role: "plan"},
			{Name: "architecture-design", Role: "plan"},
			{Name: "build", Role: "build", WritesSource: true},
			{Name: "audit", Role: "evaluate"},
		},
	}

	t.Logf("SPIKE: calling advisor.Plan() on opus, workspace=%s", ws)
	plan, err := adv.Plan(in)
	t.Logf("SPIKE RESULT: err=%v", err)
	if plan != nil {
		t.Logf("SPIKE: plan has %d entries, %d mint specs", len(plan.Entries), len(plan.MintPhases))
		for _, e := range plan.Entries {
			t.Logf("  - %s run=%v mint=%v just=%q", e.Phase, e.Run, e.Mint != nil, e.Justification)
		}
		for _, m := range plan.MintPhases {
			t.Logf("  MINT: %+v", m)
		}
	}
	if b, rerr := os.ReadFile(filepath.Join(ws, "routing-plan.json")); rerr == nil {
		t.Logf("SPIKE: routing-plan.json produced (%d bytes):\n%s", len(b), string(b))
	} else {
		t.Logf("SPIKE: NO routing-plan.json (%v)", rerr)
	}
}
