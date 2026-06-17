package router

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestPrePlanReconDigest_Deterministic pins WS2-S0b (ADR-0052): the pre-plan
// recon digest is a deterministic, sorted, fail-open function of its inputs —
// same inputs ⇒ byte-identical digest, slice fields sorted+deduped, and a nil
// changedFiles slice (an upstream git error) simply omits the file-derived facts
// rather than erroring. This is the property that lets the recon feed measured
// repo facts into the INITIAL plan without becoming a flaky or fatal dependency.
func TestPrePlanReconDigest_Deterministic(t *testing.T) {
	t.Parallel()
	files := []string{"go/internal/core/x.go", "go/internal/core/x_test.go", "web/app.ts", "go/internal/core/x.go"}
	goal := "Fix a concurrency bug in the API and add a regression test"

	a := BuildReconDigest(files, goal, 3, 2)
	b := BuildReconDigest(files, goal, 3, 2)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("BuildReconDigest not deterministic:\n%+v\n%+v", a, b)
	}
	if !sort.StringsAreSorted(a.LangsTouched) {
		t.Errorf("LangsTouched not sorted: %v", a.LangsTouched)
	}
	if !sort.StringsAreSorted(a.GoalKeywordHits) {
		t.Errorf("GoalKeywordHits not sorted: %v", a.GoalKeywordHits)
	}
	if !a.HasTests {
		t.Error("x_test.go in the change set must set HasTests")
	}
	if want := []string{"api", "bug", "concurrency", "fix", "regression", "test"}; !reflect.DeepEqual(a.GoalKeywordHits, want) {
		t.Errorf("GoalKeywordHits=%v, want %v", a.GoalKeywordHits, want)
	}
	if a.BacklogSize != 3 || a.CarryoverCount != 2 {
		t.Errorf("backlog/carryover passthrough wrong: %+v", a)
	}
	// x.go appears twice → it is the top hotspot; deduped to one entry.
	if len(a.RecentHotspots) == 0 || a.RecentHotspots[0] != "go/internal/core/x.go" {
		t.Errorf("hotspot frequency ranking wrong: %v", a.RecentHotspots)
	}

	// Fail-open: nil files omit the file-derived facts, no panic; keyword/backlog
	// facts still present, so the digest is NOT zero.
	z := BuildReconDigest(nil, goal, 0, 0)
	if len(z.LangsTouched) != 0 || z.HasTests || len(z.RecentHotspots) != 0 {
		t.Errorf("nil files must omit file facts (fail-open): %+v", z)
	}
	if z.IsZero() {
		t.Error("a digest with keyword hits must not be IsZero")
	}
	if !BuildReconDigest(nil, "", 0, 0).IsZero() {
		t.Error("empty inputs → IsZero (so render stays byte-identical)")
	}
}

// TestRenderReconDigest_FactsAndByteIdenticalOff pins the pure render: a
// populated digest renders a stable, deterministic section; a zero digest renders
// NOTHING — the property that keeps EVOLVE_ROUTER_RECON_DIGEST byte-identical
// when off (and harmless when on but nothing was gathered).
func TestRenderReconDigest_FactsAndByteIdenticalOff(t *testing.T) {
	t.Parallel()
	d := BuildReconDigest([]string{"a.go", "a_test.go"}, "fix bug", 1, 0)
	var b1, b2 strings.Builder
	RenderReconDigest(&b1, d)
	RenderReconDigest(&b2, d)
	if b1.String() != b2.String() {
		t.Error("RenderReconDigest not deterministic")
	}
	if !strings.Contains(b1.String(), "Pre-plan recon (deterministic)") {
		t.Errorf("missing heading:\n%s", b1.String())
	}

	var z strings.Builder
	RenderReconDigest(&z, ReconDigest{})
	if z.Len() != 0 {
		t.Errorf("zero digest must render nothing (byte-identical off), got:\n%q", z.String())
	}
}
