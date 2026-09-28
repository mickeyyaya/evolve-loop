package core

import (
	"testing"
)

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

	root := initDossierRepo(t)
	ws := t.TempDir()
	writeFailureArtifacts(t, ws, []string{"audit FAIL: two defects"})
	if err := writeCycleDossier(nil, cycleDossierParams{
		ProjectRoot: root, WorkspacePath: ws, Cycle: 41, Goal: "fix the mislabel", RunID: "run41", Outcome: r.FinalVerdict,
		SkippedPhases: r.SkippedPhases, VerdictsNotAdopted: r.VerdictsNotAdopted, SpineFailOpens: r.SpineFailOpens,
	}); err != nil {
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

func TestDossier_AbnormalExitStillRecordsATrueSkip(t *testing.T) {
	root := initDossierRepo(t)
	skipped := []SkippedPhase{{Phase: "closeout", Reason: "abnormal exit in phase build"}}

	if err := writeCycleDossier(nil, cycleDossierParams{
		ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 42, Goal: "died mid-build", RunID: "run42", Outcome: VerdictFAIL,
		SkippedPhases: skipped,
	}); err != nil {
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
