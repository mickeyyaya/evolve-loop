package phasespec

import (
	"strings"
	"testing"
)

func TestCatalog_Merge_OptionalBuiltinOverlayAdopted(t *testing.T) {
	merged, warns := builtinCatalogFixture().Merge([]PhaseSpec{memoLikeOverlay()})
	for _, w := range warns {
		if strings.Contains(w, "memo") && strings.Contains(w, "clashes with a built-in") {
			t.Fatalf("memo overlay (matches an OPTIONAL built-in) must not warn as a clash; got %v", warns)
		}
	}
	spec, ok := merged.Get("memo")
	if !ok {
		t.Fatal("memo must resolve in the merged catalog")
	}
	if spec.Routing == nil || len(spec.Routing.InsertWhen) == 0 {
		t.Errorf("merged memo spec must carry the overlay's routing.insert_when, not the bare built-in stub: %+v", spec)
	}
}

func TestCatalog_Merge_NonOptionalBuiltinClashStillDropped(t *testing.T) {
	hijack := PhaseSpec{Name: "audit", Optional: true, Agent: "evolve-hijack"}
	merged, warns := builtinCatalogFixture().Merge([]PhaseSpec{hijack})
	found := false
	for _, w := range warns {
		if strings.Contains(w, "audit") && strings.Contains(w, "clashes with a built-in") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an overlay hijacking the mandatory 'audit' built-in must still warn+drop; got %v", warns)
	}
	if spec, _ := merged.Get("audit"); spec.Agent == "evolve-hijack" {
		t.Error("the mandatory audit built-in must never be replaced by an overlay, even one with optional:true set on itself")
	}
}

func TestMergedCatalog_RealRepo_MemoRoutesWithoutClashWarning(t *testing.T) {
	cat, _, warns, err := MergedCatalog(repoRoot())
	if err != nil {
		t.Fatalf("MergedCatalog(repoRoot): %v", err)
	}
	for _, w := range warns {
		if strings.Contains(w, "memo") && strings.Contains(w, "clashes with a built-in") {
			t.Errorf("the real memo activation overlay must not be dropped as a built-in clash; got warning: %q", w)
		}
	}
	spec, ok := cat.Get("memo")
	if !ok {
		t.Fatal(`"memo" must resolve in the merged catalog`)
	}
	if spec.Agent != "evolve-memo" {
		t.Errorf("memo agent = %q, want the overlay's evolve-memo (proves the overlay, not the bare built-in stub, won)", spec.Agent)
	}
	if spec.Classify == nil || len(spec.Classify.RequireSections) == 0 {
		t.Error("memo's classify.require_sections must come from the overlay (Artifact Index / Skill Suggestions / carryoverTodo Guidance) — lost whenever Merge drops it as a clash")
	}
	if !spec.Optional {
		t.Error("memo must remain optional (Layer P, non-spine)")
	}
}

func TestCatalog_Merge_GenuineNewSingleWordNameUnaffected(t *testing.T) {
	widget := PhaseSpec{Name: "widget", Optional: true}
	merged, warns := builtinCatalogFixture().Merge([]PhaseSpec{widget})
	if len(warns) != 0 {
		t.Fatalf("a genuinely new user phase name (no built-in clash) must merge cleanly: %v", warns)
	}
	if _, ok := merged.Get("widget"); !ok {
		t.Fatal("widget must resolve — the exemption logic must not disturb non-clashing names")
	}
}
