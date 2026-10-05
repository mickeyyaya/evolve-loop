package core

import (
	"context"
	"strings"
	"testing"
)

func TestBuildFloorReviewer_RejectsRedSelfcheckThenApproves(t *testing.T) {
	calls := 0
	// Named binding exercises the exported check-engine type (apicover).
	var checks BuildFloorCheckFn = func(ctx context.Context, in ReviewInput) []string {
		calls++
		if calls == 1 {
			return []string{"./cmd/evolve: TestX FAIL (unit)", "gofmt: cmd_x.go"}
		}
		return nil
	}
	r := BuildHandoffFloor{{Name: "selfcheck", Run: checks}}
	in := ReviewInput{Phase: string(PhaseBuild), Worktree: "/wt", ProjectRoot: "/p"}
	res := r.Review(context.Background(), in)
	if res.Approve {
		t.Fatal("red selfcheck must reject the build deliverable")
	}
	if !res.Retry {
		t.Fatal("rejection must request the correction ladder (Retry=true)")
	}
	if !strings.Contains(res.Reason, "TestX FAIL") || !strings.Contains(res.Reason, "gofmt") {
		t.Fatalf("reason must enumerate the deterministic failures verbatim; got %q", res.Reason)
	}
	if res2 := r.Review(context.Background(), in); !res2.Approve {
		t.Fatalf("green selfcheck must approve; got %+v", res2)
	}
}

func TestBuildFloorReviewer_NonBuildPhasesUntouched(t *testing.T) {
	r := BuildHandoffFloor{{Name: "selfcheck", Run: func(context.Context, ReviewInput) []string {
		t.Fatal("selfcheck must not run for non-build phases")
		return nil
	}}}
	for _, ph := range []Phase{PhaseScout, PhaseAudit, PhaseTDD, PhaseShip} {
		if res := r.Review(context.Background(), ReviewInput{Phase: string(ph)}); !res.Approve {
			t.Fatalf("phase %s must be approved untouched", ph)
		}
	}
}

func TestBuildHandoffFloor_EmptyFloorApproves(t *testing.T) {
	if res := (BuildHandoffFloor{}).Review(context.Background(), ReviewInput{Phase: string(PhaseBuild)}); !res.Approve {
		t.Fatalf("a floor with no checks has nothing to reject; got %+v", res)
	}
}
