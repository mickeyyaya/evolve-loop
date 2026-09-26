package evalgate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestNewReviewer_TautologyEvalAdvisoryAtShadow(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "taut")
	writeEval(t, root, "taut", ":")

	got := NewReviewer(config.StageShadow).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("StageShadow must log-and-approve, never block; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

func TestNewReviewer_WeakEvalNeverBlocksAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "weak")
	writeEval(t, root, "weak", "echo checking")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("weak/advisory eval must not block even at StageEnforce; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

func TestNewReviewer_BehavioralEvalPassesAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "real")
	writeEval(t, root, "real", "go test -race ./internal/real/...")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("behavioral eval must pass at StageEnforce; got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}

func TestNewReviewer_MultipleEvalsOneTautology_BlocksNamingSlug(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "real", "taut")
	writeEval(t, root, "real", "go test -race ./internal/real/...")
	writeEval(t, root, "taut", ":")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if got.Approve {
		t.Fatalf("one tautological eval among several must still block at StageEnforce; got Approve=%v", got.Approve)
	}
	if !strings.Contains(got.Reason, "taut") {
		t.Errorf("rejection reason should name the offending slug 'taut'; got %q", got.Reason)
	}
	if strings.Contains(got.Reason, "real,") || strings.Contains(got.Reason, ", real") {
		t.Errorf("rejection reason should not implicate the clean slug 'real'; got %q", got.Reason)
	}
}

func TestNewReviewer_MissingEvalAtEnforce_QualityGateFailsOpen(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "gone")

	got := NewReviewer(config.StageEnforce).Review(context.Background(), core.ReviewInput{
		Phase: "tdd", Workspace: ws, ProjectRoot: root,
	})
	if !got.Approve {
		t.Fatalf("missing eval at phase=tdd is Gate A's (materializationGate, scout-only) job, not qualityGate's; want fail-open Approve=true, got Approve=%v Reason=%q", got.Approve, got.Reason)
	}
}
