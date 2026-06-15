package phasecontract

import (
	"strings"
	"testing"
)

// Layer 2 (ADR-0034): the Deliverable Contract is rendered into the prompt in
// two pieces — an INVARIANT instruction block (stable cache prefix, no path) and
// a VOLATILE path footer (last line; recency-optimal AND it keeps the per-cycle
// path out of the cacheable prefix).

func TestRenderContractBlock_Markdown_NoPath(t *testing.T) {
	c, _ := For("build")
	block := RenderContractBlock(c)
	if !strings.Contains(block, "## Changes") {
		t.Errorf("block must name required section %q; got:\n%s", "## Changes", block)
	}
	if !strings.Contains(block, "evolve phase verify build") {
		t.Errorf("block must instruct the self-check command; got:\n%s", block)
	}
	// Cache-safety: the invariant block must NOT embed an absolute path.
	if strings.Contains(block, "/") && strings.Contains(block, "build-report.md") {
		t.Errorf("block must not embed the artifact path (cache-safety); got:\n%s", block)
	}
}

func TestRenderContractBlock_Audit_MentionsVerdictSentinel(t *testing.T) {
	// audit is the verdict-bearing phase, so its block instructs the sentinel.
	block := RenderContractBlock(mustContract(t, "audit"))
	if !strings.Contains(block, "evolve-verdict") {
		t.Errorf("audit block must mention the verdict sentinel; got:\n%s", block)
	}
}

// Phase 3.8b (ADR-0050): build/scout/triage emit no verdict today. When the
// PhaseIO rollout activates the instruction (includePhaseIOFailureContext=true,
// i.e. EVOLVE_PHASE_IO>=advisory), their contract block gains a self-report-
// failure instruction: on a self-reported FAIL/WARN, emit a sentinel carrying a
// structured failure block. When false (the default), the block is byte-identical
// to the pre-3.8b RenderContractBlock — production (off) prompts never change.
func TestRenderContractBlockStage_BuildFailureInstructionGated(t *testing.T) {
	c := mustContract(t, "build")

	off := RenderContractBlockStage(c, false)
	if off != RenderContractBlock(c) {
		t.Fatalf("RenderContractBlockStage(c,false) must byte-equal RenderContractBlock(c)")
	}
	if strings.Contains(off, "evolve-verdict") {
		t.Errorf("default (off): build block must NOT instruct a verdict sentinel; got:\n%s", off)
	}

	on := RenderContractBlockStage(c, true)
	if !strings.Contains(on, "evolve-verdict") {
		t.Errorf("activated: build block must instruct the failure sentinel; got:\n%s", on)
	}
	if !strings.Contains(on, "FAIL") || !strings.Contains(on, "failure") {
		t.Errorf("activated: build block must explain the FAIL/WARN failure block; got:\n%s", on)
	}
}

// Audit's unconditional RequireFailureContext means its block already instructs
// the failure sentinel; the PhaseIO bool must not change or double-add it.
func TestRenderContractBlockStage_AuditUnchangedByPhaseIO(t *testing.T) {
	c := mustContract(t, "audit")
	if RenderContractBlockStage(c, false) != RenderContractBlock(c) {
		t.Errorf("audit: stage(false) must equal RenderContractBlock")
	}
	if RenderContractBlockStage(c, true) != RenderContractBlock(c) {
		t.Errorf("audit already instructs the sentinel unconditionally; the PhaseIO bool must not double-add it")
	}
}

func mustContract(t *testing.T, phase string) Contract {
	t.Helper()
	c, ok := For(phase)
	if !ok {
		t.Fatalf("no contract for %q", phase)
	}
	return c
}

func TestRenderContractBlock_Deterministic(t *testing.T) {
	c, _ := For("audit")
	if RenderContractBlock(c) != RenderContractBlock(c) {
		t.Error("RenderContractBlock must be deterministic (prompt-cache safety)")
	}
}

func TestRenderContractBlock_JSON_UsesRequiredKeys(t *testing.T) {
	// A keyed JSON contract names its required keys in the rendered block. Uses
	// orchestrator (cycle_id) — router is now a keyless bare JSON array.
	c, _ := For("orchestrator")
	block := RenderContractBlock(c)
	if !strings.Contains(block, "cycle_id") {
		t.Errorf("orchestrator JSON block must name the required key 'cycle_id'; got:\n%s", block)
	}
	if strings.Contains(block, "evolve-verdict") {
		t.Errorf("JSON deliverable has no verdict sentinel; got:\n%s", block)
	}
}

func TestRenderContractFooter_CarriesExactPath(t *testing.T) {
	c, _ := For("build")
	path := "/abs/.evolve/runs/cycle-213/build-report.md"
	footer := RenderContractFooter(c, path)
	if !strings.Contains(footer, path) {
		t.Errorf("footer must carry the exact path %q; got:\n%s", path, footer)
	}
}

// ADR-0039 §7: a RequireFailureContext contract teaches the failure-block
// emission in the SAME injected block that teaches the sentinel — one
// instruction surface for every persona/CLI, no per-agent prose copies.
func TestRenderContractBlock_FailureContextInstruction(t *testing.T) {
	audit, _ := For("audit")
	block := RenderContractBlock(audit)
	for _, want := range []string{"failure", "schema_version\":2", "\"defects\""} {
		if !strings.Contains(block, want) {
			t.Errorf("audit contract block missing failure-context teaching %q:\n%s", want, block)
		}
	}

	// Phases without the requirement keep their block free of it (cache
	// prefix stability + no irrelevant instructions).
	build, _ := For("build")
	if strings.Contains(RenderContractBlock(build), "schema_version\":2") {
		t.Error("build contract block must not carry the failure-context teaching")
	}
}
