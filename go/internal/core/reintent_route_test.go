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

	got, ok := RecordedDeferDecision(reintent)
	_, retryOK := RecordedDeferDecision(retry)
	_, noneOK := RecordedDeferDecision(t.TempDir())

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
