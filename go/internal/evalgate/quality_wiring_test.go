// quality_wiring_test.go — cycle-987 gate-wiring binding tests for the quality
// gate's presence in NewReviewer's composed gate list (reviewer.go:39).
//
// gates_test.go exercises qualityGate{}.check() DIRECTLY but never through
// NewReviewer(...).Review(...), so deleting qualityGate{} from the composition
// slice passes 100% of the existing suite while silently re-admitting
// tautological evals. These two DEFAULT-SUITE tests close that blind spot,
// mirroring TestFloorBindingGate_WiredIntoReviewer (floorbinding_test.go).

package evalgate

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestQualityGate_WiredIntoReviewer pins qualityGate ("predicate-quality") into
// the production gate list. Mirrors TestFloorBindingGate_WiredIntoReviewer:
// without this, the direct-.check() tests in gates_test.go are testing an orphan.
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

// TestNewReviewer_TautologyEvalBlocksAtEnforce drives the REAL
// NewReviewer(StageEnforce).Review() end-to-end at the tdd phase against a
// tautology (":") eval and asserts Approve==false — proving the WIRE, not just
// qualityGate{}.check(). If qualityGate were dropped from NewReviewer's slice,
// no gate would fire at tdd (materialization is scout-only, floor-binding
// fail-opens without a triage companion) and Review would approve, failing this
// test — exactly the regression the wiring test exists to catch.
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
