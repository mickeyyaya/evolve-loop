package core

import (
	"strings"
	"testing"
)

// TestPhaseAdvisor_PlanEmitsMintPhases proves the advisor can propose a NEW
// phase: an entry carrying a `mint` sub-object is mapped into
// plan.MintPhases as a phaseconfig.PhaseConfig (name from the entry, persona +
// tier + cli from the mint block), while plain run/skip entries are untouched.
func TestPhaseAdvisor_PlanEmitsMintPhases(t *testing.T) {
	t.Parallel()
	stdout := `[
	  {"phase":"scout","run":true,"justification":"fresh"},
	  {"phase":"security-sweep","run":true,"justification":"auth changed","mint":{"prompt":"You are a security reviewer. Audit the diff for authz gaps.","tier":"deep","cli":"claude","writes_source":false}}
	]`
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(baseRouteInput())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Entries) != 2 {
		t.Fatalf("entries=%d, want 2", len(plan.Entries))
	}
	if len(plan.MintPhases) != 1 {
		t.Fatalf("MintPhases=%d, want 1 (%+v)", len(plan.MintPhases), plan.MintPhases)
	}
	mc := plan.MintPhases[0]
	if mc.Name != "security-sweep" {
		t.Errorf("mint name=%q, want security-sweep", mc.Name)
	}
	if mc.Prompt == "" || !strings.Contains(mc.Prompt, "security reviewer") {
		t.Errorf("mint prompt not carried: %q", mc.Prompt)
	}
	if mc.Dispatch.ModelTierDefault != "deep" {
		t.Errorf("mint tier=%q, want deep", mc.Dispatch.ModelTierDefault)
	}
	if mc.Dispatch.CLI != "claude" {
		t.Errorf("mint cli=%q, want claude", mc.Dispatch.CLI)
	}
}

// TestPhaseAdvisor_PlanNoMint_EmptyMintPhases proves the common path is
// untouched: a plan with no mint sub-objects yields zero MintPhases (so
// registerMintedPhases is a no-op — byte-identical to pre-emit behavior).
func TestPhaseAdvisor_PlanNoMint_EmptyMintPhases(t *testing.T) {
	t.Parallel()
	stdout := `[{"phase":"scout","run":true},{"phase":"triage","run":false}]`
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(baseRouteInput())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.MintPhases) != 0 {
		t.Errorf("MintPhases=%d, want 0 for a no-mint plan", len(plan.MintPhases))
	}
}

// TestBuildPlanPrompt_DocumentsMinting proves the plan prompt teaches the
// advisor the optional mint shape (so it can actually propose new phases) and
// the tier-not-model constraint with the concrete enum, plus the mint JSON
// example — meaningful instruction, not just the bare word "mint".
func TestBuildPlanPrompt_DocumentsMinting(t *testing.T) {
	t.Parallel()
	got := buildPlanPrompt(baseRouteInput())
	for _, want := range []string{
		`"mint":{`,           // the JSON example shape
		"fast|balanced|deep", // the tier enum
		"never a raw model",  // the tier-not-model constraint
		"writes_source",      // so the advisor knows to flag source-writers
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plan prompt missing %q:\n%s", want, got)
		}
	}
}

// TestPhaseAdvisor_MintRunFalse_StillCollected proves a run:false mint entry is
// still mapped into MintPhases (registration is distinct from dispatch — the
// routing loop governs whether it runs).
func TestPhaseAdvisor_MintRunFalse_StillCollected(t *testing.T) {
	t.Parallel()
	stdout := `[{"phase":"deferred-probe","run":false,"justification":"reserve","mint":{"prompt":"probe persona","tier":"fast"}}]`
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(baseRouteInput())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.MintPhases) != 1 || plan.MintPhases[0].Name != "deferred-probe" {
		t.Errorf("run:false mint must still be collected; got %+v", plan.MintPhases)
	}
}

// TestPhaseAdvisor_PlanMintCarriesSelectMetadata proves the minter satisfies the
// catalog SELECT-metadata contract itself (cycle-1275): description/when_to_use
// supplied in the advisor's mint block land on the minted PhaseSpec, which is
// exactly what TestPhaseCatalog_OptionalPhasesHaveSelectMetadata reads. Before
// this, every minted phase reached the catalog metadata-less and the gate was
// satisfied by padding metadataAllowlist after the fact (#404, #406).
func TestPhaseAdvisor_PlanMintCarriesSelectMetadata(t *testing.T) {
	t.Parallel()
	stdout := `[{"phase":"schema-drift-check","run":true,"justification":"wire types changed","mint":{"prompt":"drift persona","tier":"balanced","description":"Reports wire-schema drift.","when_to_use":"Select when router wire structs change."}}]`
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(baseRouteInput())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.MintPhases) != 1 {
		t.Fatalf("MintPhases=%d, want 1", len(plan.MintPhases))
	}
	mc := plan.MintPhases[0]
	if mc.Description != "Reports wire-schema drift." {
		t.Errorf("minted Description=%q, want the advisor's value", mc.Description)
	}
	if mc.WhenToUse != "Select when router wire structs change." {
		t.Errorf("minted WhenToUse=%q, want the advisor's value", mc.WhenToUse)
	}
}

// TestPhaseAdvisor_PlanMintWithoutMetadata_StaysEmpty pins backward
// compatibility: a mint block with no metadata keys mints exactly as before —
// empty metadata, every other field carried, no error.
func TestPhaseAdvisor_PlanMintWithoutMetadata_StaysEmpty(t *testing.T) {
	t.Parallel()
	stdout := `[{"phase":"legacy-probe","run":true,"mint":{"prompt":"legacy persona","tier":"deep"}}]`
	plan, err := NewPhaseAdvisor(&fakeBridge{stdout: stdout}).Plan(baseRouteInput())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.MintPhases) != 1 {
		t.Fatalf("MintPhases=%d, want 1", len(plan.MintPhases))
	}
	if mc := plan.MintPhases[0]; mc.Description != "" || mc.WhenToUse != "" {
		t.Errorf("metadata-less mint invented metadata: %q / %q", mc.Description, mc.WhenToUse)
	}
	if plan.MintPhases[0].Prompt != "legacy persona" {
		t.Errorf("legacy mint prompt regressed: %q", plan.MintPhases[0].Prompt)
	}
}

// TestPlanPrompt_DocumentsMintMetadata proves the advisor is INSTRUCTED to
// supply the metadata — an unadvertised field is never emitted. Both
// prompt-assembly paths must document it: composePlanPrompt (persona,
// PRODUCTION) and buildPlanPrompt (legacy inline fallback), the pair that
// diverged at #293.
func TestPlanPrompt_DocumentsMintMetadata(t *testing.T) {
	t.Parallel()
	persona := NewPhaseAdvisor(&fakeBridge{}, WithPersona("You are the evolve router."))
	prompts := map[string]string{
		"legacy":  buildPlanPrompt(baseRouteInput()),
		"persona": persona.composePlanPrompt(baseRouteInput(), "routing-plan.json"),
	}
	for name, got := range prompts {
		for _, want := range []string{`"description"`, `"when_to_use"`} {
			if !strings.Contains(got, want) {
				t.Errorf("%s plan prompt missing %s — advisor never told to supply SELECT metadata", name, want)
			}
		}
	}
}
