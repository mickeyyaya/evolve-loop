package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// hasNotAdopted reports whether a phase's declined verdict was preserved in
// CycleResult.VerdictsNotAdopted.
func hasNotAdopted(recs []VerdictNotAdopted, phase string) bool {
	for _, s := range recs {
		if s.Phase == phase {
			return true
		}
	}
	return false
}

func TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict(t *testing.T) {
	o := &Orchestrator{}
	r := &CycleResult{}

	o.recordFinalVerdict(r, PhaseAudit, VerdictPASS, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit"}))
	if r.FinalVerdict != VerdictPASS {
		t.Fatalf("after audit PASS: FinalVerdict=%q, want PASS", r.FinalVerdict)
	}

	o.recordFinalVerdict(r, PhaseRetro, VerdictFAIL, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit", "ship", "retro"}))
	if r.FinalVerdict != VerdictPASS {
		t.Errorf("non-floor retro FAIL clobbered floor verdict: FinalVerdict=%q, want PASS (the storm)", r.FinalVerdict)
	}
	if !hasNotAdopted(r.VerdictsNotAdopted, string(PhaseRetro)) {
		t.Errorf("retro degrade not recorded in VerdictsNotAdopted: %+v", r.VerdictsNotAdopted)
	}
}

func TestNonFloorPhaseFailure_FailAudit_StaysFail(t *testing.T) {
	o := &Orchestrator{}
	r := &CycleResult{}

	o.recordFinalVerdict(r, PhaseAudit, VerdictFAIL, o.floorAlreadyCompleted([]string{"tdd", "build", "audit"}))
	if r.FinalVerdict != VerdictFAIL {
		t.Fatalf("after audit FAIL: FinalVerdict=%q, want FAIL", r.FinalVerdict)
	}

	o.recordFinalVerdict(r, PhaseRetro, VerdictFAIL, o.floorAlreadyCompleted([]string{"tdd", "build", "audit", "retro"}))
	if r.FinalVerdict != VerdictFAIL {
		t.Errorf("non-floor retro must not change a FAIL cycle: FinalVerdict=%q, want FAIL", r.FinalVerdict)
	}
}

func TestFloorPhaseFailure_RemainsCycleFatal(t *testing.T) {
	o := &Orchestrator{}

	for _, phase := range []Phase{PhaseAudit, PhaseBuild, PhaseTDD, PhaseShip} {
		if !o.isAuthoritativePhase(phase) {
			t.Errorf("phase %q must be authoritative (floor/ship)", phase)
		}
		r := &CycleResult{FinalVerdict: VerdictPASS}
		o.recordFinalVerdict(r, phase, VerdictFAIL, o.floorAlreadyCompleted([]string{"tdd", "build", "audit"}))
		if r.FinalVerdict != VerdictFAIL {
			t.Errorf("floor phase %q FAIL must be cycle-fatal: FinalVerdict=%q, want FAIL", phase, r.FinalVerdict)
		}
	}

	if o.isAuthoritativePhase(PhaseRetro) {
		t.Errorf("retro must not be authoritative — it is a post-verdict phase")
	}
}

func TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict(t *testing.T) {
	o := &Orchestrator{}
	r := &CycleResult{FinalVerdict: VerdictPASS}
	priorSessionLog := []string{"scout", "tdd", "build", "audit", "ship", "retro"}

	o.recordFinalVerdict(r, PhaseRetro, VerdictFAIL, o.floorAlreadyCompleted(priorSessionLog))
	if r.FinalVerdict != VerdictPASS {
		t.Errorf("resume: non-floor retro FAIL clobbered a persisted floor PASS: FinalVerdict=%q, want PASS", r.FinalVerdict)
	}
	if !hasNotAdopted(r.VerdictsNotAdopted, string(PhaseRetro)) {
		t.Errorf("resume: retro degrade not recorded in VerdictsNotAdopted: %+v", r.VerdictsNotAdopted)
	}
}

func TestContractExhaustion_NonFloorPhase_DegradesToSkippedWarn(t *testing.T) {
	o := &Orchestrator{}
	ws := t.TempDir()

	floorDone := o.floorAlreadyCompleted([]string{"tdd", "build", "audit"})
	degraded, ok := o.nonFloorExhaustionDegrade(PhaseRetro, ws, floorDone)
	if !ok {
		t.Fatalf("non-floor retro exhaustion (post-floor) must degrade, got ok=false")
	}
	if degraded.Verdict != VerdictSKIPPED {
		t.Errorf("degraded verdict=%q, want SKIPPED", degraded.Verdict)
	}
	if degraded.ArtifactsDir != ws {
		t.Errorf("degraded response must carry the workspace dir, got %q", degraded.ArtifactsDir)
	}

	// The degrade then flows through recordFinalVerdict as a non-clobbering
	// verdict-not-adopted entry (floor already passed).
	r := &CycleResult{FinalVerdict: VerdictPASS}
	o.recordFinalVerdict(r, PhaseRetro, degraded.Verdict, o.floorAlreadyCompleted([]string{"tdd", "build", "audit", "retro"}))
	if r.FinalVerdict != VerdictPASS || !hasNotAdopted(r.VerdictsNotAdopted, string(PhaseRetro)) {
		t.Errorf("degraded retro must preserve PASS and record the declined verdict: verdict=%q notAdopted=%+v", r.FinalVerdict, r.VerdictsNotAdopted)
	}

	// A floor phase's exhaustion stays fatal (no degrade path).
	if _, ok := o.nonFloorExhaustionDegrade(PhaseAudit, ws, floorDone); ok {
		t.Errorf("floor phase audit exhaustion must NOT degrade — it stays cycle-fatal")
	}
	// A non-floor phase BEFORE the floor (scout) stays fatal too — you cannot
	// proceed on an unparseable scout verdict.
	if _, ok := o.nonFloorExhaustionDegrade(PhaseScout, ws, o.floorAlreadyCompleted(nil)); ok {
		t.Errorf("pre-floor scout exhaustion must NOT degrade — no floor verdict exists yet")
	}
}

func TestDossier_RecordsSkippedPhases(t *testing.T) {
	notAdopted := []VerdictNotAdopted{{Phase: "retrospective", Verdict: VerdictFAIL}}
	skipped := []SkippedPhase{{Phase: "closeout", Reason: "abnormal exit in phase build"}}
	d, err := dossier.Build(9, dossier.BuildOpts{
		WorkspacePath:      t.TempDir(),
		Goal:               "cycle-802 floor-gated verdict",
		FinalVerdict:       VerdictPASS,
		SkippedPhases:      skipped,
		VerdictsNotAdopted: notAdopted,
	})
	if err != nil {
		t.Fatalf("dossier.Build: %v", err)
	}
	if len(d.PhasesRunVerdictNotAdopted) != 1 ||
		d.PhasesRunVerdictNotAdopted[0].Phase != "retrospective" ||
		d.PhasesRunVerdictNotAdopted[0].Verdict != VerdictFAIL {
		t.Errorf("dossier did not surface the ran-but-declined retro verdict: %+v", d.PhasesRunVerdictNotAdopted)
	}
	if len(d.SkippedPhases) != 1 || d.SkippedPhases[0].Phase != "closeout" {
		t.Errorf("dossier did not surface a genuine skip: %+v", d.SkippedPhases)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("dossier with both record kinds must still validate: %v", err)
	}
}
