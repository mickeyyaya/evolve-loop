package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestDeferDecision_ReadsTheFixTypeOfADeferDecisionOnly(t *testing.T) {
	t.Parallel()
	reintent := cycle1953Workspace(t, "intent-malformed", true)
	retry := t.TempDir()
	if err := os.WriteFile(filepath.Join(retry, "failure-decision.json"), []byte(`{"category":"code-audit-fail","level":"task","action":"retry-with-fix","fix_type":"address-audit-findings"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	o := floorOrchestrator(nil)
	got, ok := o.deferDecision(CycleState{WorkspacePath: reintent})
	_, retryOK := o.deferDecision(CycleState{WorkspacePath: retry})
	_, noneOK := o.deferDecision(CycleState{WorkspacePath: t.TempDir()})

	if !ok || got.category != "intent-malformed" || got.fixType != policy.FixTypeReintent {
		t.Errorf("deferDecision(cycle 1953) = %+v, %t; want intent-malformed with fix_type reintent", got, ok)
	}
	if retryOK || noneOK {
		t.Errorf("a retry-with-fix decision (%t) or no decision (%t) gave a defer route", retryOK, noneOK)
	}
}

func TestDeferDecision_TheAuditBlockWinsOverTheRetroDecision(t *testing.T) {
	t.Parallel()
	alone := cycle1953Workspace(t, "intent-malformed", false)
	disagree := cycle1953Workspace(t, "intent-malformed", false)
	if err := os.WriteFile(filepath.Join(disagree, "failure-decision.json"), []byte(`{"category":"infra-systemic","level":"task","action":"defer-or-quarantine","fix_type":"pipeline-repair"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o := floorOrchestrator(nil)

	for name, ws := range map[string]string{"the audit block alone": alone, "a retro decision that disagrees": disagree} {
		ended, defers := o.auditDeclineDefers(CycleState{WorkspacePath: ws, AuditDeclineReason: "declined"})
		if !defers || ended.category != "intent-malformed" || ended.fixType != policy.FixTypeReintent || ended.source != "audit" {
			t.Errorf("%s: the cycle ended on %+v %t; want the audit's intent-malformed with fix_type reintent", name, ended, defers)
		}
	}
}
