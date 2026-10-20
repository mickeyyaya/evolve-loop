package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRecordedDeferDecision_ReadsTheFixTypeOfADeferDecisionOnly(t *testing.T) {
	t.Parallel()
	reintent := cycle1953Workspace(t, "intent-malformed", true)
	retry := t.TempDir()
	if err := os.WriteFile(filepath.Join(retry, "failure-decision.json"), []byte(`{"category":"code-audit-fail","level":"task","action":"retry-with-fix","fix_type":"address-audit-findings"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	pol := policy.DefaultSystemFailurePolicy()
	got, ok := RecordedDeferDecision(reintent, pol)
	_, retryOK := RecordedDeferDecision(retry, pol)
	_, noneOK := RecordedDeferDecision(t.TempDir(), pol)

	if !ok || got != (RecordedDefer{Category: "intent-malformed", FixType: policy.FixTypeReintent}) {
		t.Errorf("RecordedDeferDecision(cycle 1953) = %+v, %t; want intent-malformed with fix_type reintent", got, ok)
	}
	if retryOK || noneOK {
		t.Errorf("a retry-with-fix decision (%t) or no decision (%t) gave a defer route", retryOK, noneOK)
	}
}

func TestIsAuditDeferResult_IsTheRetroDecisionOfADeferEnd(t *testing.T) {
	t.Parallel()
	d := deferDecision{category: "intent-malformed", fixType: "reintent", source: "failure-decision.json"}
	for _, c := range []struct {
		decision string
		want     bool
	}{
		{d.reason(), true},
		{"phase-gate: retro artifact missing; " + d.reason(), true},
		{"proceed: no retry", false},
		{"", false},
	} {
		if got := IsAuditDeferResult(CycleResult{RetroDecision: c.decision}); got != c.want {
			t.Errorf("IsAuditDeferResult(%q) = %t, want %t", c.decision, got, c.want)
		}
	}
}

func TestRecordedDeferDecision_TheAuditBlockWinsAsItDoesWhenTheCycleEnds(t *testing.T) {
	t.Parallel()
	pol := policy.DefaultSystemFailurePolicy()
	alone := cycle1953Workspace(t, "intent-malformed", false)
	disagree := cycle1953Workspace(t, "intent-malformed", false)
	if err := os.WriteFile(filepath.Join(disagree, "failure-decision.json"), []byte(`{"category":"infra-systemic","level":"task","action":"defer-or-quarantine","fix_type":"pipeline-repair"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o := floorOrchestrator(nil)

	for name, ws := range map[string]string{"the audit block alone": alone, "a retro decision that disagrees": disagree} {
		got, ok := RecordedDeferDecision(ws, pol)
		ended, defers := o.auditDeclineDefers(CycleState{WorkspacePath: ws, AuditDeclineReason: "declined"})
		if !ok || got != (RecordedDefer{Category: "intent-malformed", FixType: policy.FixTypeReintent}) || !defers || ended.fixType != got.FixType {
			t.Errorf("%s: RecordedDeferDecision = %+v, %t; the cycle ended on %+v: want the audit's intent-malformed with fix_type reintent for both", name, got, ok, ended)
		}
	}
}
