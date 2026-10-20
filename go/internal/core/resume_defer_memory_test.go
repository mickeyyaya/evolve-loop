package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func plantDecision(t *testing.T, ws string) string {
	t.Helper()
	path := filepath.Join(ws, failureDecisionFile)
	if err := os.WriteFile(path, []byte(cycle1953FailureDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func retroSeesDecision(path string, seen *bool) func(PhaseRequest) {
	return func(PhaseRequest) {
		_, err := os.Stat(path)
		*seen = err == nil
	}
}

func TestDispatch_TheRetroNeverSeesADecisionPlantedBeforeIt(t *testing.T) {
	t.Parallel()
	seen := true
	retro := &resumeHookRunnerE4{name: "retro", verdict: VerdictFAIL}
	cr := dispatchRunE4(t, &fakeLedger{}, retro, PhaseRetro, 1)
	retro.hook = retroSeesDecision(plantDecision(t, cr.cs.WorkspacePath), &seen)

	if _, _, err := cr.dispatch(PhaseRetro); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(retro.requests) != 1 || seen {
		t.Errorf("retro runs = %d, planted decision present at the retro = %t; want the host to clear it before the live retro", len(retro.requests), seen)
	}
}

func TestResumeExecution_AResumedRetroNeverSeesADecisionPlantedBeforeIt(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := plantDecision(t, ws)
	seen := true
	_, runners, o := resumeRigE4(t, ws, func(cs *CycleState) { cs.AuditDeclineReason = "the retry budget is spent" })
	runners[PhaseRetro] = &resumeHookRunnerE4{name: "retro", verdict: VerdictFAIL, hook: retroSeesDecision(path, &seen)}

	res, err := resumeFromE4(o, PhaseRetro)

	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if seen || strings.Contains(res.RetroDecision, auditDeferReasonPrefix) {
		t.Errorf("planted decision present at the retro = %t, decision = %q; want a resumed retro to clear the planted decision so it never defers", seen, res.RetroDecision)
	}
}

func TestResumeExecution_TheDeferThatEndsAResumedCycleIsCarriedOnTheResult(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	_, runners, o := resumeRigE4(t, ws, func(cs *CycleState) { cs.AuditDeclineReason = "the retry budget is spent" })
	runners[PhaseRetro] = &resumeHookRunnerE4{name: "retro", verdict: VerdictFAIL, hook: func(PhaseRequest) { plantDecision(t, ws) }}

	res, err := resumeFromE4(o, PhaseRetro)

	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if d, ok := DeferOf(res); !ok || d != (RecordedDefer{Category: "intent-malformed", FixType: "reintent"}) {
		t.Errorf("DeferOf = %+v, %t (decision %q); want the resumed retro's defer carried for the closeout", d, ok, res.RetroDecision)
	}
}

func TestCarryDefer_ADeferReasonWithNoResolvableDeferWarns(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	o := floorOrchestrator(nil)
	o.stderr = &errOut
	var res CycleResult

	o.carryDefer(CycleState{CycleID: 9, WorkspacePath: t.TempDir()}, auditDeferReasonPrefix+"gone", &res)

	if _, ok := DeferOf(res); ok || !strings.Contains(errOut.String(), "WARN cycle 9") {
		t.Errorf("result = %+v, stderr = %q; want no carried defer and a loud WARN", res, errOut.String())
	}
}

func TestWriteDossier_APlantedSymlinkIsReplacedAndItsTargetIsUntouched(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "host-state.json")
	if err := os.WriteFile(outside, []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{CycleID: 9, WorkspacePath: t.TempDir()}
	link := filepath.Join(cs.WorkspacePath, failureDossierFile)
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	floorOrchestrator(nil).writeDossier(cs, &failureDossier{CycleID: 9})

	target, _ := os.ReadFile(outside)
	info, err := os.Lstat(link)
	if string(target) != "ORIGINAL" || err != nil || !info.Mode().IsRegular() {
		t.Errorf("target = %q, dossier entry = %v (err %v); want the symlink replaced by a regular file and its target untouched", target, info, err)
	}
}

func TestWriteDossier_ADossierThatCannotBeWrittenWarns(t *testing.T) {
	t.Parallel()
	var errOut bytes.Buffer
	o := floorOrchestrator(nil)
	o.stderr = &errOut
	cs := CycleState{CycleID: 9, WorkspacePath: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(cs.WorkspacePath, failureDossierFile, "planted"), 0o755); err != nil {
		t.Fatal(err)
	}

	o.writeDossier(cs, &failureDossier{CycleID: 9})

	if !strings.Contains(errOut.String(), "WARN cycle 9: "+failureDossierFile+" not written") {
		t.Errorf("stderr = %q; want a WARN for a dossier that cannot replace the planted entry", errOut.String())
	}
}
