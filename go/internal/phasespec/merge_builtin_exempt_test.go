package phasespec

// merge_builtin_exempt_test.go — RED contract for cycle-554's
// memo-phase-routing-restore task.
//
// CORRECTED ROOT CAUSE (surfaces a Rule-3 pivot from the scout-report framing
// — documented in test-report.md): scout proposed renaming the built-in
// "memo" entry to a multi-word name. Investigation of the ACTUAL code found
// cycle-547 already shipped the real fix's foundation:
// ValidateUserSpecWithCatalog + ApplyUserRouting(3-arg) both exempt a
// single-word overlay name that matches an OPTIONAL built-in (see
// validate_builtin_exempt_test.go) — exactly the "memo" shape. That rollout
// missed ONE call site: Catalog.Merge (discover.go), which MergedCatalog
// (mergedcatalog.go — "the ONE merged-catalog loader", consumed by the CLI,
// self-check, host-contract-gate, and runner default) still calls unqualified.
// Verified empirically against the real repo today:
//
//	cat, _, warns, _ := MergedCatalog(repoRoot())
//	// warns == ["user phase memo clashes with a built-in — built-in kept, ..."]
//	// cat.Get("memo") == the bare built-in stub: Agent="", Classify=nil
//
// i.e. the overlay's agent/classify/routing fields never reach the merged
// Catalog (only cfg.Order/Triggers get them, via ApplyUserRouting operating
// on the raw discovered specs) — so any consumer resolving the phase THROUGH
// the catalog (CLI `phase list`, self-check, host-contract-gate) sees a
// hollow stub. Renaming the registry entry would ABANDON cycle-547's
// already-tested exemption machinery for a single-slice workaround
// (never_duplicate_centralize_via_design_patterns); completing the SAME
// exemption into Merge is the minimal, non-duplicative fix — reusing
// isOptionalBuiltinName (validate.go), not inventing a second mechanism.
// The reserved single-word floor is untouched: Merge's new adoption path is
// scoped exactly to isOptionalBuiltinName's existing safety scope (matches an
// OPTIONAL built-in only; a NON-optional built-in clash — e.g. "audit" —
// still drops with the existing warning, the anti-hijack negative case).
//
// No migration of .evolve/policy.json's pins.memo is needed: the phase keeps
// its name "memo" (this fix does not rename anything).

import (
	"strings"
	"testing"
)

// TestCatalog_Merge_OptionalBuiltinOverlayAdopted — the core fix: an overlay
// whose name clashes with an OPTIONAL built-in (the memo shape) must be
// ADOPTED into the merged catalog, not dropped — carrying its routing fields.
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

// TestCatalog_Merge_NonOptionalBuiltinClashStillDropped — NEGATIVE/anti-hijack:
// an overlay named after a mandatory (non-optional) spine phase must still be
// dropped with the clash warning, even if the overlay marks itself
// optional:true. Mirrors TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay
// at the Merge layer — the cheapest gaming fake ("always adopt on any name
// clash") fails this test.
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

// TestMergedCatalog_RealRepo_MemoRoutesWithoutClashWarning — the actual
// end-to-end reproduction against the real repo config
// (docs/architecture/phase-registry.json + .evolve/phases/memo/phase.json):
// the single-loader every real consumer uses must resolve memo's overlay
// fields, not the hollow built-in stub.
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

// TestCatalog_Merge_GenuineNewSingleWordNameUnaffected — EDGE: a user phase
// name that does not clash with ANY built-in is untouched by the new
// exemption path (it never enters the isBuiltin branch at all) — pins that
// the fix is additive, not a behavior change for the common case.
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
