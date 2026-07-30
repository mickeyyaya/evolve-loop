package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

// final_verdict_floor_test.go — cycle-802 (retro-bridge-timeout-width10)
// acceptance tests. These are the NAMED internal/core unit tests the ACS
// predicates in go/acs/cycle802 invoke as a `-race` subprocess. Each exercises
// the floor-gated FinalVerdict guard (final_verdict_floor.go) — the fix for the
// storm where a non-floor retro/memo FAIL clobbered an audit PASS.
//
// The completed-phase log passed to floorAlreadyCompleted mirrors the two write
// sites (cyclerun_record.go / resume.go), where CompletedPhases already includes
// the phase being recorded.

// hasNotAdopted reports whether a phase's declined verdict was preserved. The
// records live in CycleResult.VerdictsNotAdopted (dossier
// phases_run_verdict_not_adopted); they used to be filed under SkippedPhases,
// which made the dossier claim a phase that RAN had been skipped
// (dossier-retro-skipped-mislabel). What cycle-802 requires is unchanged: the
// degrade is PRESERVED, never dropped.
func hasNotAdopted(recs []VerdictNotAdopted, phase string) bool {
	for _, s := range recs {
		if s.Phase == phase {
			return true
		}
	}
	return false
}

// AC1: audit PASS, then a non-floor phase (retro) FAILs under quota/timeout —
// FinalVerdict must stay PASS and retro's declined verdict must be surfaced.
func TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict(t *testing.T) {
	o := &Orchestrator{}
	r := &CycleResult{}

	// audit (floor) records PASS.
	o.recordFinalVerdict(r, PhaseAudit, VerdictPASS, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit"}))
	if r.FinalVerdict != VerdictPASS {
		t.Fatalf("after audit PASS: FinalVerdict=%q, want PASS", r.FinalVerdict)
	}

	// retro (non-floor) FAILs after the floor already passed.
	o.recordFinalVerdict(r, PhaseRetro, VerdictFAIL, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit", "ship", "retro"}))
	if r.FinalVerdict != VerdictPASS {
		t.Errorf("non-floor retro FAIL clobbered floor verdict: FinalVerdict=%q, want PASS (the storm)", r.FinalVerdict)
	}
	if !hasNotAdopted(r.VerdictsNotAdopted, string(PhaseRetro)) {
		t.Errorf("retro degrade not recorded in VerdictsNotAdopted: %+v", r.VerdictsNotAdopted)
	}
}

// AC2: audit already FAILed, then a non-floor phase FAILs — FinalVerdict must
// stay FAIL (the guard must not accidentally "recover" a failed cycle either).
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

// AC3 (negative control): a FLOOR phase's own failure must remain cycle-fatal.
// The guard scopes to non-floor phases only — it must not shield audit/build/tdd
// or ship from setting a FAIL verdict.
func TestFloorPhaseFailure_RemainsCycleFatal(t *testing.T) {
	o := &Orchestrator{}

	for _, phase := range []Phase{PhaseAudit, PhaseBuild, PhaseTDD, PhaseShip} {
		if !o.isAuthoritativePhase(phase) {
			t.Errorf("phase %q must be authoritative (floor/ship)", phase)
		}
		r := &CycleResult{FinalVerdict: VerdictPASS}
		// Even with a prior PASS on the log, a floor phase's own FAIL sets FAIL.
		o.recordFinalVerdict(r, phase, VerdictFAIL, o.floorAlreadyCompleted([]string{"tdd", "build", "audit"}))
		if r.FinalVerdict != VerdictFAIL {
			t.Errorf("floor phase %q FAIL must be cycle-fatal: FinalVerdict=%q, want FAIL", phase, r.FinalVerdict)
		}
	}

	// And a non-floor phase must NOT be authoritative (proves the scope split).
	if o.isAuthoritativePhase(PhaseRetro) {
		t.Errorf("retro must not be authoritative — it is a post-verdict phase")
	}
}

// AC4: resume-path parity. A cycle resumed mid-recovery — audit already PASSed
// and was persisted to CompletedPhases in the PRIOR session — must apply the
// identical guard when the resumed retro FAILs. resume.go computes
// floorAlreadyCompleted from the persisted log exactly as tested here.
func TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict(t *testing.T) {
	o := &Orchestrator{}
	// Resume re-enters with a floor verdict already established across sessions.
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

// AC5: contract exhaustion (unparseable verdict after retries) on a non-floor
// phase degrades to SKIPPED+WARN and advances, whereas a floor phase stays
// cycle-fatal.
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

// AC6: degraded/skipped non-floor phases are surfaced in the dossier, never
// dropped. The record is SPLIT BY WHAT ACTUALLY HAPPENED (dossier-retro-skipped-
// mislabel): a phase that RAN and had its verdict declined goes to
// CycleResult.VerdictsNotAdopted → phases_run_verdict_not_adopted, while
// skipped_phases keeps its literal meaning for a phase that did not run.
// Cycle-802's requirement — the outcome survives instead of clobbering the floor
// verdict — is what both halves preserve.
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
