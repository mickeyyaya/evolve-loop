package policy

import (
	"encoding/json"
	"testing"
)

func TestGatesConfig_ReportSizeGate_DefaultsShadow(t *testing.T) {
	if got := (Policy{}).GatesConfig().ReportSizeGate; got != "shadow" {
		t.Errorf("default ReportSizeGate = %q, want %q (shadow/warn first per the inbox spec)", got, "shadow")
	}
}

func TestGatesConfig_ReportSizeGate_ExplicitOverrideHonored(t *testing.T) {
	got := (Policy{Gates: &GatesPolicy{ReportSizeGate: "enforce"}}).GatesConfig()
	if got.ReportSizeGate != "enforce" {
		t.Errorf("ReportSizeGate override = %q, want %q", got.ReportSizeGate, "enforce")
	}
	if got.ContractGate != "enforce" || got.EvalGate != "enforce" || got.TriageCapGate != "enforce" {
		t.Errorf("setting ReportSizeGate alone must not disturb the other gate defaults; got %+v", got)
	}
}

func TestReportBudgetConfig_DefaultsTo2000(t *testing.T) {
	if got := (Policy{}).ReportBudgetConfig().HandoffTokens; got != 2000 {
		t.Errorf("default HandoffTokens = %d, want 2000 (the inbox item's stated ~2K default)", got)
	}
	if got := (Policy{ReportBudget: &ReportBudgetPolicy{}}).ReportBudgetConfig().HandoffTokens; got != 2000 {
		t.Errorf("an empty (but present) ReportBudget block must still resolve the 2000 default, got %d", got)
	}
}

func TestReportBudgetConfig_ExplicitOverrideHonored(t *testing.T) {
	got := (Policy{ReportBudget: &ReportBudgetPolicy{HandoffTokens: 500}}).ReportBudgetConfig()
	if got.HandoffTokens != 500 {
		t.Errorf("HandoffTokens override = %d, want 500 (policy-sourced, not a Go literal)", got.HandoffTokens)
	}
}

func TestReportBudgetPolicy_JSONRoundTrip(t *testing.T) {
	data := []byte(`{"gates":{"report_size_gate":"enforce"},"report_budget":{"handoff_tokens":777}}`)
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal policy.json gates/report_budget block: %v", err)
	}
	if p.Gates == nil || p.Gates.ReportSizeGate != "enforce" {
		t.Fatalf("Gates after unmarshal = %+v, want ReportSizeGate=enforce", p.Gates)
	}
	if p.ReportBudget == nil || p.ReportBudget.HandoffTokens != 777 {
		t.Fatalf("ReportBudget after unmarshal = %+v, want HandoffTokens=777", p.ReportBudget)
	}
}
