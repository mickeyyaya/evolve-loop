package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestNewDefaultWithStage_NamedPhase names the concrete audit.Phase type
// (New/NewDefault return *Phase but the type is never named in a test) and
// exercises NewDefaultWithStage — the composition-root seam that threads the
// EVOLVE_PHASE_IO stage into verdict extraction (ADR-0050 §3.10 Slice 5).
func TestNewDefaultWithStage_NamedPhase(t *testing.T) {
	br := &fakeBridge{}
	prm := fakePromptsFS("# Auditor body")

	// Contract 1: NewDefaultWithStage returns a runnable *Phase with the "audit"
	// identity that satisfies the embedded core.PhaseRunner.
	var enforce *Phase = NewDefaultWithStage(br, prm, config.StageEnforce)
	if enforce == nil {
		t.Fatal("NewDefaultWithStage must return a non-nil *Phase")
	}
	var _ core.PhaseRunner = enforce
	if got := enforce.Name(); got != string(core.PhaseAudit) {
		t.Fatalf("Name() = %q, want %q", got, core.PhaseAudit)
	}

	// Contract 2: the StageOff convenience constructor returns the same runnable
	// identity (byte-identical legacy path).
	off := NewDefaultWithStage(br, prm, config.StageOff)
	if off == nil || off.Name() != string(core.PhaseAudit) {
		t.Fatalf("StageOff NewDefaultWithStage Name() = %q, want audit", off.Name())
	}

	// Contract 3: the stage is the gate that drives sentinel-mandatory grading.
	// A prose-only report (no evolve-verdict sentinel) is read as PASS below
	// enforce but is unparseable AT enforce — exactly the stage NewDefaultWithStage
	// wired into the phase's hooks.
	prose := "## Verdict\n**PASS**\n"
	if v, found := extractAuditVerdict(prose, config.StageOff); !found || v != core.VerdictPASS {
		t.Errorf("StageOff: prose verdict = (%q,%v), want (PASS,true)", v, found)
	}
	if v, found := extractAuditVerdict(prose, config.StageEnforce); found {
		t.Errorf("StageEnforce: prose-only report must be unparseable (sentinel mandatory), got (%q,%v)", v, found)
	}
}

// TestNewDefaultWithStageCompact_NamedPhase names + exercises
// NewDefaultWithStageCompact — the compact-prompts seam added in cycle 413
// (workflow.compact_prompts) that shipped without an apicover naming test and
// reddened main CI (apicover -enforce: "UNCOVERED (no test names it)"). Beyond
// the name gate, it pins the constructor's reason to exist: compact=true threads
// prompts.StripOnDemandSections into the dispatch path so the on-demand reference
// tail never reaches the model, while compact=false leaves the body intact.
func TestNewDefaultWithStageCompact_NamedPhase(t *testing.T) {
	// Contract 1: the constructor returns a runnable *Phase with audit identity.
	var compactPhase *Phase = NewDefaultWithStageCompact(&fakeBridge{}, fakePromptsFS("# Auditor body"), config.StageEnforce, true)
	if compactPhase == nil {
		t.Fatal("NewDefaultWithStageCompact must return a non-nil *Phase")
	}
	var _ core.PhaseRunner = compactPhase
	if got := compactPhase.Name(); got != string(core.PhaseAudit) {
		t.Fatalf("Name() = %q, want %q", got, core.PhaseAudit)
	}

	// Contract 2: compact=true strips the on-demand reference tail from the
	// DISPATCHED prompt; compact=false leaves it. Driven end-to-end through the
	// fake bridge, which captures the request the adapter would materialize. The
	// production "## Reference Index (Layer 3, on-demand)" heading form is used so
	// this also guards the cycle-413 prefix-match fix at the dispatch boundary.
	const body = "# Auditor body\n\n## Reference Index (Layer 3, on-demand)\n\n- tail-only reference content\n"
	dispatchedPrompt := func(compact bool) string {
		fb := &fakeBridge{writeArtifact: "## Verdict\n**PASS**\n"}
		ph := New(Config{Bridge: fb, Prompts: fakePromptsFS(body), CompactPrompts: compact})
		if _, err := ph.Run(context.Background(), core.PhaseRequest{
			Cycle: 1, ProjectRoot: t.TempDir(), Worktree: t.TempDir(), Workspace: t.TempDir(),
		}); err != nil {
			t.Fatalf("Run(compact=%v): %v", compact, err)
		}
		return fb.gotReq.Prompt
	}

	full := dispatchedPrompt(false)
	if !strings.Contains(full, "Auditor body") {
		t.Fatalf("compact=false must keep the body; dispatched prompt=%q", full)
	}
	if !strings.Contains(full, "Reference Index") || !strings.Contains(full, "tail-only reference content") {
		t.Fatalf("compact=false must keep the on-demand tail; dispatched prompt=%q", full)
	}

	stripped := dispatchedPrompt(true)
	if !strings.Contains(stripped, "Auditor body") {
		t.Fatalf("compact=true must keep the body above the tail; dispatched prompt=%q", stripped)
	}
	if strings.Contains(stripped, "Reference Index") || strings.Contains(stripped, "tail-only reference content") {
		t.Fatalf("compact=true must strip the on-demand tail before dispatch; dispatched prompt=%q", stripped)
	}
}
