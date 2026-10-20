package core

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestFenceFor_IsTheOneProjection(t *testing.T) {
	t.Parallel()
	writable := []string{"go/internal/x.go"}

	if f := FenceFor(true, writable); f != nil {
		t.Errorf("FenceFor(writes) = %+v, want nil: a phase that writes source is not fenced", f)
	}
	if f := FenceFor(false, nil); f == nil || len(f.Writable) != 0 {
		t.Errorf("FenceFor(read-only) = %+v, want a fence with no writable path", f)
	}
	f := FenceFor(false, writable)
	if f == nil || !slices.Equal(f.Writable, writable) {
		t.Fatalf("FenceFor(read-only, writable) = %+v, want the fence carrying the writable paths", f)
	}
	writable[0] = "changed"
	if f.Writable[0] != "go/internal/x.go" {
		t.Error("the fence shares its writable slice with the caller")
	}
}

func TestOutOfBandRetro_IsFencedLikeEveryReadOnlyPhase(t *testing.T) {
	t.Parallel()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")

	_, _ = h.cr.reviewWithCorrections(PhaseBuild, h.dr)

	retro := h.cr.o.runners[PhaseRetro].(*fakeRunner)
	if len(retro.requests) != 1 {
		t.Fatalf("retro dispatched %d time(s), want 1", len(retro.requests))
	}
	if req := retro.requests[0]; req.Fence == nil || req.WritesSource() {
		t.Fatalf("retro request fence = %+v; the out-of-band retro runs read-only, so a writing agent's edits are reverted", req.Fence)
	}
}

func TestWorktreeFence_WritablePathsOfNoFenceIsNone(t *testing.T) {
	t.Parallel()
	var none *WorktreeFence
	if got := none.WritablePaths(); got != nil {
		t.Errorf("(*WorktreeFence)(nil).WritablePaths() = %v, want nil: an unfenced request lists no writable paths", got)
	}
	if got := (&WorktreeFence{Writable: []string{"a.go"}}).WritablePaths(); !slices.Equal(got, []string{"a.go"}) {
		t.Errorf("WritablePaths() = %v, want [a.go]", got)
	}
}

func TestPhaseWritesSource_TheBuiltinWritersOrACatalogSpecThatDeclaresIt(t *testing.T) {
	t.Parallel()
	cat, warnings := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{{Name: "minted-writer", WritesSource: true}, {Name: "minted-reader"}})
	if len(warnings) != 0 {
		t.Fatalf("catalog: %v", warnings)
	}
	for phase, want := range map[Phase]bool{PhaseBuild: true, PhaseAudit: false, "minted-writer": true, "minted-reader": false, "unknown": false} {
		if got := PhaseWritesSource(phase, cat); got != want {
			t.Errorf("PhaseWritesSource(%s) = %v, want %v", phase, got, want)
		}
	}
}
