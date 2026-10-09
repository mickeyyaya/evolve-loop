package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeACSVerdictShip(t *testing.T, ws string, redCount int, ship *bool) {
	t.Helper()
	v := map[string]any{
		"cycle":      42,
		"red_count":  redCount,
		"total":      10,
		"predicates": []any{},
	}
	if ship != nil {
		v["ship_eligible"] = *ship
	}
	b, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
		t.Fatalf("write verdict: %v", err)
	}
}

func normalPassBridge() *fakeBridge {
	return &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
}

func TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled(t *testing.T) {
	ws := t.TempDir()
	no := false
	writeACSVerdictShip(t, ws, 0, &no)
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("a normally-completed phase returns nil error even when it FAILs; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL — a narrative PASS with acs-verdict ship_eligible:false must not ship on the normal-completion path", resp.Verdict)
	}
}

func TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 2)
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("normal completion returns nil error even when it FAILs; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL — PASS report with red_count>0 must not ship", resp.Verdict)
	}
}

func TestRun_NormalCompletion_PassReport_ShipEligibleTrue_StaysPass(t *testing.T) {
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS — red_count:0 + ship_eligible:true is a genuine PASS and must not be spuriously rejected", resp.Verdict)
	}
}

func TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictShip(t, ws, 0, nil)
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS — a legacy verdict without a ship_eligible field must pass on red_count:0 (back-compat)", resp.Verdict)
	}
}
