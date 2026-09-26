package topngate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewReviewer_EnforceApprovesLabelDrift(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("label drift must approve (advisory); got reason=%q", res.Reason)
	}
}

func TestNewReviewer_EnforceApprovesInLaneBuild(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "statefile-rmw-flock-single-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Errorf("enforce must approve an in-lane build; got reason=%q", res.Reason)
	}
}

func TestNewReviewer_ShadowLogsButApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageShadow)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("shadow must approve even on violation; got Approve=false (%s)", res.Reason)
	}
}

func TestNewReviewer_NonBuildPhaseApproves(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	r := NewReviewer(config.StageEnforce)
	if res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseAudit), Workspace: ws}); !res.Approve {
		t.Errorf("gate must not apply to phase audit; want approve, got reason=%q", res.Reason)
	}
}

func TestNewReviewer_TDDEnforceBlocksEmptyTopN(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws)
	writeTDDReport(t, ws, "orphan-task-cycle-1113", "go/acs/cycle1113/predicates_test.go")
	res := NewReviewer(config.StageEnforce).Review(
		context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if res.Approve {
		t.Fatalf("enforce must BLOCK orphan TDD authoring under an EMPTY ## top_n; got Approve=true reason=%q", res.Reason)
	}
	if res.Reason == "" {
		t.Errorf("a blocked review must record a non-empty abort_reason — it is the operator's only evidence")
	}
	if !strings.Contains(res.Reason, "orphan-task-cycle-1113") {
		t.Errorf("abort reason must name the claimed slug; got %q", res.Reason)
	}
	if !strings.Contains(res.Reason, "go/acs/cycle1113/predicates_test.go") {
		t.Errorf("abort reason must name the authored file(s) so the operator can find the orphan scaffold; got %q", res.Reason)
	}
}

func TestNewReviewer_TDDShadowApprovesEmptyTopN(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws)
	writeTDDReport(t, ws, "orphan-task-cycle-1113", "go/acs/cycle1113/predicates_test.go")
	res := NewReviewer(config.StageShadow).Review(
		context.Background(), core.ReviewInput{Phase: string(core.PhaseTDD), Workspace: ws})
	if !res.Approve {
		t.Fatalf("shadow must approve even the FATAL empty-top_n case; got Approve=false reason=%q", res.Reason)
	}
}

func TestReplayCycle640Shape(t *testing.T) {
	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws})
	if !res.Approve {
		t.Fatalf("label drift is advisory since 2026-07-22; got reason=%q", res.Reason)
	}
}
