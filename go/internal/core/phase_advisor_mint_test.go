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
