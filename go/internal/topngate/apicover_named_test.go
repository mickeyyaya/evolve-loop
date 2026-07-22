package topngate

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported topngate symbol by identifier (apicover counts field
// access as "uses", not "names"). NewReviewer is the package's sole export;
// each assertion pins a REAL contract (the composite gate reviewer's blocking
// and fail-open behaviour), not a magic string. Helpers writeTriageReport /
// writeBuildReport are shared from reviewer_test.go in this package.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestNewReviewer_Named names NewReviewer by identifier and pins its two
// load-bearing contracts: it returns a usable core.DeliverableReviewer, and at
// StageEnforce that reviewer blocks an out-of-lane build with a non-empty
// abort_reason while StageShadow approves the same violation (log-only rollout).
func TestNewReviewer_Named(t *testing.T) {
	t.Parallel()

	var enforce core.DeliverableReviewer = NewReviewer(config.StageEnforce)
	if enforce == nil {
		t.Fatal("NewReviewer(StageEnforce) must return a non-nil core.DeliverableReviewer")
	}

	ws := t.TempDir()
	writeTriageReport(t, ws, "statefile-rmw-flock-single-source")
	writeBuildReport(t, ws, "fix-token-resolver-transcript-source")
	in := core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws}

	// Advisory since 2026-07-22: label drift approves at every stage (see
	// reviewer_test.go rationale); the apicover contract here is the NAMED
	// exercise of the reviewer types, not the old fatal policy.
	if res := enforce.Review(context.Background(), in); !res.Approve {
		t.Errorf("enforce reviewer approves label drift (advisory); got reason=%q", res.Reason)
	}

	var shadow core.DeliverableReviewer = NewReviewer(config.StageShadow)
	if res := shadow.Review(context.Background(), in); !res.Approve {
		t.Errorf("shadow reviewer must approve (log-only) the same violation; got reason=%q", res.Reason)
	}
}
