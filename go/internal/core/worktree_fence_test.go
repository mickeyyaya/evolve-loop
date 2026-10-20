package core

import (
	"encoding/json"
	"slices"
	"strings"
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

func TestPhaseRequestJSON_TheFenceCrossesABinaryOnEitherSideOfTheKeyRename(t *testing.T) {
	t.Parallel()
	fenced := PhaseRequest{Worktree: "/wt", Fence: &WorktreeFence{Writable: []string{"a.go"}}}
	raw, err := json.Marshal(fenced)
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		ReadOnly bool     `json:"worktree_read_only"`
		Writable []string `json:"worktree_writable_paths"`
	}
	if err := json.Unmarshal(raw, &old); err != nil || !old.ReadOnly || !slices.Equal(old.Writable, []string{"a.go"}) {
		t.Fatalf("legacy view of %s = %+v, %v; a binary built before the rename must still see the fence", raw, old, err)
	}
	for name, body := range map[string]string{
		"the new key":     string(raw),
		"the legacy keys": `{"worktree":"/wt","worktree_read_only":true,"worktree_writable_paths":["a.go"]}`,
	} {
		var got PhaseRequest
		if err := json.Unmarshal([]byte(body), &got); err != nil || got.Fence == nil || !slices.Equal(got.Fence.Writable, []string{"a.go"}) || got.Worktree != "/wt" {
			t.Errorf("%s: decoded %+v (fence %+v), %v; the fence must survive either spelling", name, got, got.Fence, err)
		}
	}
	var open PhaseRequest
	if err := json.Unmarshal([]byte(`{"worktree":"/wt"}`), &open); err != nil || open.Fence != nil {
		t.Errorf("a request with no fence key = %+v, %v; want unfenced", open.Fence, err)
	}
	if raw, _ := json.Marshal(PhaseRequest{Worktree: "/wt"}); strings.Contains(string(raw), "worktree_read_only") {
		t.Errorf("an unfenced request marshals %s; it must not claim read-only", raw)
	}
}

func TestPhaseRequest_MarshalJSONAndUnmarshalJSONKeepTheFenceAndRefuseBadJSON(t *testing.T) {
	t.Parallel()
	raw, err := PhaseRequest{Worktree: "/wt", Fence: &WorktreeFence{}}.MarshalJSON()
	if err != nil || !strings.Contains(string(raw), `"worktree_read_only":true`) || !strings.Contains(string(raw), `"worktree_fence":{}`) {
		t.Fatalf("MarshalJSON = %s, %v; want both the new and the legacy fence keys", raw, err)
	}
	var back PhaseRequest
	if err := back.UnmarshalJSON(raw); err != nil || back.Fence == nil {
		t.Fatalf("UnmarshalJSON = %+v, %v; want the fence back", back.Fence, err)
	}
	var mixed PhaseRequest
	if err := mixed.UnmarshalJSON([]byte(`{"worktree_fence":{"writable":["x.go"]},"worktree_read_only":true,"worktree_writable_paths":["y.go"]}`)); err != nil || mixed.Fence == nil || !slices.Equal(mixed.Fence.Writable, []string{"x.go"}) {
		t.Fatalf("UnmarshalJSON = %+v, %v; when both spellings disagree, the new key wins", mixed.Fence, err)
	}
	if err := back.UnmarshalJSON([]byte("{not json")); err == nil {
		t.Fatal("UnmarshalJSON of broken JSON must fail")
	}
}
