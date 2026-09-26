package evalgate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestQualityGate_WiredIntoReviewer(t *testing.T) {
	found := false
	for _, g := range newGatesForTest() {
		if g.name() == "predicate-quality" {
			found = true
		}
	}
	if !found {
		t.Fatal("qualityGate is not wired into NewReviewer's gate list")
	}
}

func TestNewReviewer_TautologyEvalBlocksAtEnforce(t *testing.T) {
	ws, root := t.TempDir(), t.TempDir()
	writeScoutReport(t, ws, "taut")
	writeEval(t, root, "taut", ":") // no-op tautology → LevelHalt

	r := NewReviewer(config.StageEnforce)
	res := r.Review(context.Background(), core.ReviewInput{Phase: "tdd", Workspace: ws, ProjectRoot: root})
	if res.Approve {
		t.Fatalf("enforce reviewer must reject a tautology eval at tdd; got Approve=true")
	}
	if !strings.Contains(res.Reason, "taut") {
		t.Errorf("reject reason should name the tautology slug; got %q", res.Reason)
	}
}
