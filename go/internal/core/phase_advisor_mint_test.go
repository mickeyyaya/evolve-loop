package core

import (
	"strings"
	"testing"
)

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
