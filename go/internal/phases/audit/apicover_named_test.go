package audit

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
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
