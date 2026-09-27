package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// writeACSVerdictShip writes acs-verdict.json with an explicit red_count and an
// OPTIONAL ship_eligible field. ship==nil omits the field entirely (the legacy
// shape writeACSVerdict produces) so a test can pin back-compat: audit must not
// require a field older verdicts never carried.
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

// normalPassBridge returns a fakeBridge that COMPLETES normally (no timeout, no
// error) and writes a narrative-PASS audit report — the opposite of the
// auditTimeoutErr() bridges the reconcile-on-timeout tests use. It exercises the
// happy Classify path where the auditor's own report is authoritative.
func normalPassBridge() *fakeBridge {
	return &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"}
}

// TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled:
// when the auditor completes normally, writes a narrative PASS, but
// acs-verdict.json — the acssuite SSOT — says ship_eligible:false, the phase
// must reject (FAIL/WARN), never accept the narrative uncontested.
func TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled(t *testing.T) {
	ws := t.TempDir()
	no := false
	writeACSVerdictShip(t, ws, 0, &no) // red_count==0 but SSOT says do-not-ship
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("a normally-completed phase returns nil error even when it FAILs; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL — a narrative PASS with acs-verdict ship_eligible:false must not ship on the normal-completion path", resp.Verdict)
	}
}

// TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail materializes
// the headline case on the normal-completion path (the reconcile tests
// elsewhere only cover the exit-81 timeout path): a narrative PASS with red
// predicates must FAIL.
func TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 2) // two red predicates, no ship_eligible field
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("normal completion returns nil error even when it FAILs; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL — PASS report with red_count>0 must not ship", resp.Verdict)
	}
}

// TestRun_NormalCompletion_PassReport_ShipEligibleTrue_StaysPass is the
// negative test: genuine agreement must pass through untouched, ruling out
// the cheapest fix of always downgrading to FAIL regardless of the ACS
// verdict.
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

// TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass pins
// back-compat: a verdict that omits ship_eligible (every verdict written
// before the field existed) with red_count:0 must still PASS — the
// ship_eligible read is not wired as a mandatory field.
func TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdictShip(t, ws, 0, nil) // field absent — legacy shape
	phase := New(Config{Bridge: normalPassBridge(), Prompts: fakePromptsFS("body")})

	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS — a legacy verdict without a ship_eligible field must pass on red_count:0 (back-compat)", resp.Verdict)
	}
}
