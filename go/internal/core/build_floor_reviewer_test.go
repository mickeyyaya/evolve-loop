package core

// build_floor_reviewer_test.go — the shift-left half of the 2026-07-21
// operator directive ("shift deterministic gate to the front as part of build
// phase verification"): the build deliverable is REJECTED while the changed
// packages' deterministic self-check fails, so the EXISTING E2 correction
// ladder fixes it in-phase — instead of the defect surfacing at a later gate
// (or worse: cycle-1008's builder recorded ./cmd/evolve failing in
// build-selfcheck.json and handed off anyway; the FAIL then cost 4 more
// phases + the cycle).

import (
	"context"
	"strings"
	"testing"
)

func TestBuildFloorReviewer_RejectsRedSelfcheckThenApproves(t *testing.T) {
	calls := 0
	r := NewBuildFloorReviewer(func(ctx context.Context, in ReviewInput) []string {
		calls++
		if calls == 1 {
			return []string{"./cmd/evolve: TestX FAIL (unit)", "gofmt: cmd_x.go"}
		}
		return nil
	})
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
	r := NewBuildFloorReviewer(func(context.Context, ReviewInput) []string {
		t.Fatal("selfcheck must not run for non-build phases")
		return nil
	})
	for _, ph := range []Phase{PhaseScout, PhaseAudit, PhaseTDD, PhaseShip} {
		if res := r.Review(context.Background(), ReviewInput{Phase: string(ph)}); !res.Approve {
			t.Fatalf("phase %s must be approved untouched", ph)
		}
	}
}

func TestBuildFloorReviewer_ChecksErrorFailsOpen(t *testing.T) {
	// A selfcheck INFRASTRUCTURE error (cannot even run) must fail OPEN with a
	// loud reason-free approve — the deterministic gates downstream stay armed;
	// the floor must never false-block a build over its own plumbing.
	r := NewBuildFloorReviewer(nil) // nil fn = engine unavailable
	if res := r.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild)}); !res.Approve {
		t.Fatalf("nil engine must fail open; got %+v", res)
	}
}
