package triagecap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// newTestReviewer builds the clamp with seam overrides: a fixed package
// vocabulary and a fixed throughput window (no real state.json needed).
func newTestReviewer(stage config.Stage, window []core.TriageThroughputEntry, logs *[]string) *CapReviewer {
	r := newCapReviewer(stage)
	r.pkgsFn = func(string) []string { return knownPkgsFixture }
	r.windowFn = func(string) []core.TriageThroughputEntry { return window }
	r.failsFn = func(string) []FailEntry { return nil }
	if logs != nil {
		r.logf = func(f string, a ...any) { *logs = append(*logs, f) }
	} else {
		r.logf = func(string, ...any) {}
	}
	return r
}

func writeTriageWorkspace(t *testing.T, artifact string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func reviewIn(ws string) core.ReviewInput {
	return core.ReviewInput{Phase: "triage", Workspace: ws, ProjectRoot: ws}
}

// TestCapReviewer_Cycle283ShapeRejected — the R9.2 acceptance replay: the
// overpacked cycle-283 artifact (12 floors) against the empty-window seed
// (K=5, cap=7) must reject at enforce with an actionable cap directive.
func TestCapReviewer_Cycle283ShapeRejected(t *testing.T) {
	ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle283.md"))
	r := newTestReviewer(config.StageEnforce, nil, nil)
	rr := r.Review(context.Background(), reviewIn(ws))
	if rr.Approve {
		t.Fatal("cycle-283 shape (12 floors > cap 7) must be rejected at enforce")
	}
	for _, want := range []string{"12", "7", "## deferred"} {
		if !strings.Contains(rr.Reason, want) {
			t.Errorf("reject reason missing %q: %s", want, rr.Reason)
		}
	}
}

// TestCapReviewer_Cycle281ShapePasses — the PASS baseline (1 floor) is
// approved untouched.
func TestCapReviewer_Cycle281ShapePasses(t *testing.T) {
	ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle281.md"))
	r := newTestReviewer(config.StageEnforce, nil, nil)
	if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
		t.Errorf("cycle-281 shape must pass: %s", rr.Reason)
	}
}

// TestCapReviewer_ObservedWindowTightensCap: a window of lean cycles lowers
// K below the seed — the clamp follows observed throughput, not the constant.
func TestCapReviewer_ObservedWindowTightensCap(t *testing.T) {
	// K = mean(2,2,2) = 2 → cap = ceil(2.5) = 3; a 4-floor commit rejects.
	window := []core.TriageThroughputEntry{
		{Cycle: 290, Floors: 2}, {Cycle: 291, Floors: 2}, {Cycle: 292, Floors: 2},
	}
	artifact := "## top_n\n- coverage-four: swarmrunner, swarmplan, bridge, recovery coverage ≥98%\n"
	ws := writeTriageWorkspace(t, artifact)
	r := newTestReviewer(config.StageEnforce, window, nil)
	if rr := r.Review(context.Background(), reviewIn(ws)); rr.Approve {
		t.Error("4 floors > cap 3 (observed K=2) must reject")
	}
}

func TestCapReviewer_NonTriagePhaseApproved(t *testing.T) {
	r := newTestReviewer(config.StageEnforce, nil, nil)
	rr := r.Review(context.Background(), core.ReviewInput{Phase: "build", Workspace: t.TempDir()})
	if !rr.Approve {
		t.Error("non-triage phases are out of scope for the capacity clamp")
	}
}

func TestCapReviewer_ShadowLogsWouldBlockAndApproves(t *testing.T) {
	ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle283.md"))
	var logs []string
	r := newTestReviewer(config.StageShadow, nil, &logs)
	if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
		t.Fatal("shadow stage must approve")
	}
	found := false
	for _, l := range logs {
		if strings.Contains(l, "would-block") {
			found = true
		}
	}
	if !found {
		t.Error("shadow stage must log the would-block")
	}
}

func TestCapReviewer_MissingArtifactFailsOpen(t *testing.T) {
	r := newTestReviewer(config.StageEnforce, nil, nil)
	if rr := r.Review(context.Background(), reviewIn(t.TempDir())); !rr.Approve {
		t.Error("missing artifact is ambiguity — fail open")
	}
}

func TestCapReviewer_OffStageApproves(t *testing.T) {
	ws := writeTriageWorkspace(t, readFixture(t, "triage-cycle283.md"))
	r := newTestReviewer(config.StageOff, nil, nil)
	if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
		t.Error("off stage must approve")
	}
}
