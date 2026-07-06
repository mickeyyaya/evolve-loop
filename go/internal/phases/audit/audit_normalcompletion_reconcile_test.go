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

// TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled is
// the GENUINE RED for this cycle (auditor-egps-reconciliation-gate, criterion 1).
//
// The auditor completes NORMALLY (no bridge timeout), writes a narrative PASS,
// but acs-verdict.json — the acssuite SSOT — says ship_eligible:false. The phase
// MUST reject (FAIL/WARN), never accept the agent's PASS uncontested.
//
// Why this is red today: audit's Classify reads ONLY red_count (readRedCount) as
// a proxy for ship-eligibility. When the authoritative ship_eligible flag says
// "not shippable" while red_count happens to be 0 (the two can diverge — a
// pre-staged/agent-written verdict, or a future acssuite that gates on more than
// the red count), the false PASS slips straight through. The fix reads the
// authoritative ship_eligible field on the normal-completion path, symmetric to
// the timeout path (both route through Classify — no duplicate branch).
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

// TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail materialises
// criterion 1's headline case on the NORMAL-completion path (the existing
// reconcile tests only cover the exit-81 timeout path). A narrative PASS with
// red predicates must FAIL. Pre-existing GREEN: Classify's red_count gate
// already covers this — the test locks it against regression.
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

// TestRun_NormalCompletion_PassReport_ShipEligibleTrue_StaysPass is the negative
// test (criterion 2): genuine agreement must pass through untouched. It kills the
// cheapest gaming fix — always downgrading to FAIL regardless of the ACS verdict.
// Pre-existing GREEN; must STAY green after the ship_eligible read lands.
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

// TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass pins criterion
// 3 (edge/back-compat): a verdict that OMITS ship_eligible (every verdict written
// before this cycle) with red_count:0 must still PASS — no panic, no spurious
// reject. This is the guard that stops the ship_eligible read from being wired as
// a mandatory-field requirement that would false-FAIL every legacy verdict.
// Pre-existing GREEN; must STAY green after the fix.
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
