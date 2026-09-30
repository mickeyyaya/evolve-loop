package topngate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeLaneScope(t *testing.T, workspace string, ids ...string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"todo_ids": ids, "goal_hash": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "lane-scope.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTDDScopeGate_ReconcilesFromDecisionWithoutTriageReport(t *testing.T) {
	ws := t.TempDir()
	writeTriageDecision(t, ws, []string{"alpha-task", "beta-task"}, nil)
	writeScopedTDDReport(t, ws, []string{"alpha-task"}, "")

	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "scope-mismatch") || !strings.Contains(got.Reason, "beta-task") {
		t.Fatalf("a two-member commitment recorded only in triage-decision.json must still block a one-member TDD declaration, naming the omitted member; got %+v", got)
	}
}

func TestTDDScopeGate_ReconcilesFromLaneScopeWithoutTriageReportOrDecision(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, "alpha-task", "beta-task")
	writeScopedTDDReport(t, ws, []string{"beta-task"}, "")

	got := reviewTDD(t, ws)
	if got.Approve || !strings.Contains(got.Reason, "alpha-task") {
		t.Fatalf("a pinned lane with only lane-scope.json must be reconciled and block the omitted member; got %+v", got)
	}
}

func TestTDDScopeGate_MatchingDeclarationWithoutTriageReportApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriageDecision(t, ws, []string{"alpha-task", "beta-task"}, nil)
	writeScopedTDDReport(t, ws, []string{"alpha-task", "beta-task"}, "")

	if got := reviewTDD(t, ws); !got.Approve {
		t.Fatalf("a complete declaration must approve even without triage-report.md; got %+v", got)
	}
}

func TestTDDScopeGate_NoCommitmentRecordAtAllFailsLoud(t *testing.T) {
	ws := t.TempDir()
	writeScopedTDDReport(t, ws, []string{"alpha-task"}, "")

	got := reviewTDD(t, ws)
	if got.Approve {
		t.Fatalf("triage-report.md, triage-decision.json and lane-scope.json all missing is an unverifiable scope, not an approval; got %+v", got)
	}
	if !strings.Contains(got.Reason, "triage-report.md") {
		t.Errorf("the block must name the missing commitment record so the operator can act; reason=%q", got.Reason)
	}
}

func TestReviewer_GateAppliesToTDDOnlyNotOtherPhases(t *testing.T) {
	ws := t.TempDir()
	r := NewReviewer(config.StageEnforce)
	for _, phase := range []core.Phase{core.PhaseScout, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
		if res := r.Review(context.Background(), core.ReviewInput{Phase: string(phase), Workspace: ws}); !res.Approve {
			t.Errorf("the scope gate must not apply to phase %s (no commitment record is fine there); got reason=%q", phase, res.Reason)
		}
	}
}
