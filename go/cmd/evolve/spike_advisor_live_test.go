//go:build spike

// Live diagnostic spike (not in normal CI; run with -tags spike). Calls the
// PhaseAdvisor's Plan() directly with the REAL bridge on opus, to answer:
// does the LLM advisor actually produce a routing plan, or does it error/degrade?
// Run: go test ./cmd/evolve/ -tags spike -run TestSpikeAdvisorLive -v -timeout 300s
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func TestSpikeAdvisorLive(t *testing.T) {
	root, err := filepath.Abs("../../..") // repo root: go/cmd/evolve → cmd → go → repo
	if err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	cfg, _ := config.Load(registry, filterEvolveEnv(os.Environ()))

	br := bridge.NewDefault(root)
	cli, model := resolveRouterDispatch(filepath.Join(root, ".evolve"))
	var persona string
	if rp, perr := newPromptsLoader(root).Agent("evolve-router"); perr == nil {
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
	// This test never fails — it is a diagnostic that reports what happened.
}
