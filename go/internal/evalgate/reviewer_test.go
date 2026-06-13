package evalgate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// newTestReviewer builds the composite with a capturing logger.
func newTestReviewer(stage config.Stage, log *[]string) *reviewer {
	return &reviewer{
		stage: stage,
		gates: []gate{materializationGate{}, qualityGate{}},
		logf:  func(f string, a ...any) { *log = append(*log, strings.TrimSpace(fmt.Sprintf(f, a...))) },
	}
}

func TestReviewer_EnforceBlocksMissingEval(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "needs-eval") // no eval file written
	var log []string
	r := newTestReviewer(config.StageEnforce, &log)
	res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
	if res.Approve {
		t.Fatalf("enforce must reject a missing-eval scout; got Approve=true")
	}
	if !strings.Contains(res.Reason, "needs-eval") {
		t.Errorf("reason should name the slug; got %q", res.Reason)
	}
	if len(log) == 0 {
		t.Error("a violation should be logged")
	}
}

func TestReviewer_ShadowLogsButApproves(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "needs-eval")
	var log []string
	r := newTestReviewer(config.StageShadow, &log)
	res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
	if !res.Approve {
		t.Fatalf("shadow must approve even on violation; got Approve=false (%s)", res.Reason)
	}
	if len(log) == 0 {
		t.Error("shadow must still log the violation")
	}
}

func TestReviewer_OffNeverBlocks(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "needs-eval")
	var log []string
	r := newTestReviewer(config.StageOff, &log)
	if res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root}); !res.Approve {
		t.Errorf("StageOff must never block; got Approve=false (%s)", res.Reason)
	}
}

func TestReviewer_NonGatedPhaseApproves(t *testing.T) {
	var log []string
	r := newTestReviewer(config.StageEnforce, &log)
	// build phase: neither gate applies → approve, nothing logged.
	if res := r.Review(context.Background(), core.ReviewInput{Phase: "build", Workspace: t.TempDir(), ProjectRoot: t.TempDir()}); !res.Approve {
		t.Errorf("no gate applies to build; want approve")
	}
	if len(log) != 0 {
		t.Errorf("no gate applies to build; nothing should be logged, got %v", log)
	}
}

func TestReviewer_HealthyCycleApprovesAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "a")
	writeEval(t, root, "a", "go test ./internal/a/...")
	var log []string
	r := newTestReviewer(config.StageEnforce, &log)
	if res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root}); !res.Approve {
		t.Errorf("a healthy scout must approve at enforce; got %s", res.Reason)
	}
}

func TestNewReviewer_ProducesWorkingReviewer(t *testing.T) {
	r := NewReviewer(config.StageEnforce)
	if r == nil {
		t.Fatal("NewReviewer returned nil")
	}
	if res := r.Review(context.Background(), core.ReviewInput{Phase: "ship"}); !res.Approve {
		t.Error("ship phase (no gate) must approve")
	}
}

func TestNewReviewer_ShadowViolationExercisesDefaultLogger(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "needs-eval")
	r := NewReviewer(config.StageShadow)
	res := r.Review(context.Background(), core.ReviewInput{Phase: "scout", Workspace: ws, ProjectRoot: root})
	if !res.Approve {
		t.Fatalf("shadow reviewer must approve while logging violation; got %q", res.Reason)
	}
}
