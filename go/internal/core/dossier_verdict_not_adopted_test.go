package core

// dossier_verdict_not_adopted_test.go — dossier-retro-skipped-mislabel. Every
// FAIL dossier from cycles 1028/1035 and 1105-1117 (and on through 1198) carried
//
//	"skipped_phases": [{"phase": "retro", "reason": "FAIL"}]
//
// while the run dir held a 15-24KB retrospective-report.md: retro RAN. The record
// contradicted its own artifacts, which is worse than no record — the disposition
// assembler and recurrence analysis read dossiers to learn which judgment phases
// executed.
//
// Mechanism (final_verdict_floor.go): recordFinalVerdict appends to the list when
// a non-floor phase that RAN returns non-PASS after the floor verdict is set, so
// the field always meant "verdict not adopted" and Reason carried the VERDICT, not
// a skip cause. The fix is naming/semantics — those records get their own field
// (phases_run_verdict_not_adopted) and skipped_phases is left to phases that
// genuinely did not run (the abnormal-epilogue closeout entry). NOT "make retro
// run on FAIL": it already does, and that edit would break the cycle-802 clobber
// guard this code exists to enforce.

import (
	"testing"
)

// TestDossier_RetroThatRanIsNotRecordedAsSkipped is the RED: the exact production
// chain of a FAIL cycle whose non-floor retro ran and returned FAIL —
// recordFinalVerdict → writeCycleDossier → knowledge-base/cycles/cycle-N.json.
// retro must not appear under skipped_phases; it must appear as a phase that RAN
// whose verdict was not adopted.
func TestDossier_RetroThatRanIsNotRecordedAsSkipped(t *testing.T) {
	o := &Orchestrator{}
	r := &CycleResult{}
	// audit (floor) records FAIL, then retro RUNS and also reports FAIL.
	o.recordFinalVerdict(r, PhaseAudit, VerdictFAIL, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit"}))
	o.recordFinalVerdict(r, PhaseRetro, VerdictFAIL, o.floorAlreadyCompleted([]string{"scout", "tdd", "build", "audit", "retro"}))

	if len(r.SkippedPhases) != 0 {
		t.Errorf("retro RAN — it must not be recorded as a skipped phase; SkippedPhases=%+v", r.SkippedPhases)
	}
	if len(r.VerdictsNotAdopted) != 1 ||
		r.VerdictsNotAdopted[0].Phase != string(PhaseRetro) ||
		r.VerdictsNotAdopted[0].Verdict != VerdictFAIL {
		t.Fatalf("the non-adopted retro verdict must be recorded (never dropped — cycle-802); got %+v", r.VerdictsNotAdopted)
	}

	root := t.TempDir()
	initDossierRepo(t, root)
	ws := t.TempDir()
	writeFailureArtifacts(t, ws, []string{"audit FAIL: two defects"})
	if err := writeCycleDossier(nil, root, ws, 41, "fix the mislabel", "run41", r.FinalVerdict,
		r.SkippedPhases, r.VerdictsNotAdopted, r.SpineFailOpens); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	m, _ := readDossierPair(t, root, 41)

	if _, present := m["skipped_phases"]; present {
		t.Errorf("the committed dossier must NOT claim a phase was skipped when it ran; skipped_phases=%v", m["skipped_phases"])
	}
	recs, ok := m["phases_run_verdict_not_adopted"].([]any)
	if !ok || len(recs) != 1 {
		t.Fatalf("the dossier must record the ran-but-not-adopted retro; keys=%v", dossierTopLevelKeys(m))
	}
	rec, ok := recs[0].(map[string]any)
	if !ok {
		t.Fatalf("phases_run_verdict_not_adopted[0] is not an object: %v", recs[0])
	}
	if rec["phase"] != string(PhaseRetro) || rec["verdict"] != VerdictFAIL {
		t.Errorf("phases_run_verdict_not_adopted[0] = %v, want {phase: retro, verdict: FAIL} — the field names the VERDICT, which is what the old reason: FAIL always was", rec)
	}
}

// TestDossier_AbnormalExitStillRecordsATrueSkip is the scope guard: skipped_phases
// keeps its literal meaning for the one writer that produces a REAL skip — the
// abnormal epilogue, where closeout never ran because the cycle died mid-phase.
// Emptying the field entirely would lose that.
func TestDossier_AbnormalExitStillRecordsATrueSkip(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	skipped := []SkippedPhase{{Phase: "closeout", Reason: "abnormal exit in phase build"}}

	if err := writeCycleDossier(nil, root, t.TempDir(), 42, "died mid-build", "run42", VerdictFAIL,
		skipped, nil, nil); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	m, _ := readDossierPair(t, root, 42)

	recs, ok := m["skipped_phases"].([]any)
	if !ok || len(recs) != 1 {
		t.Fatalf("a genuinely skipped phase must still be recorded under skipped_phases; keys=%v", dossierTopLevelKeys(m))
	}
	rec, ok := recs[0].(map[string]any)
	if !ok || rec["phase"] != "closeout" || rec["reason"] != "abnormal exit in phase build" {
		t.Errorf("skipped_phases[0] = %v, want the closeout skip with its skip CAUSE", recs[0])
	}
	if _, present := m["phases_run_verdict_not_adopted"]; present {
		t.Errorf("no phase ran-but-unadopted here; the field must be omitted, not empty: %v", m["phases_run_verdict_not_adopted"])
	}
}
