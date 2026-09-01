package core

// audit_round_artifacts_test.go — regression contract for the cycle-1603
// stale-verdict-artifact class (2026-09-02).
//
// Any audit RE-dispatch (the ADR-0092/0093 repair loop, a bookkeeping regrade,
// ship-error recovery, debugger RERUN_PHASE) that leaves the previous round's
// verdict artifacts at their canonical paths replays superseded evidence
// through audit.Classify's verdict-exists gate: in cycle-1603 round-1's
// agent-amended ship_eligible=false forced every repaired PASS back to FAIL,
// so the repair loop was structurally unable to succeed. The retirement lives
// at the audit pre-dispatch seam (beside resetFloorFailReason, both dispatch
// surfaces) — the unit tests pin the helper's contract, and the live test pins
// the wiring through a real repair cycle.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRetireSupersededAuditArtifacts_ArchivesBothVerdictFiles(t *testing.T) {
	dir := t.TempDir()
	acsBody := `{"verdict":"PASS","red_count":0,"ship_eligible":false,"audit_verdict":"FAIL"}`
	reportBody := "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n"
	mustWriteRoundArtifact(t, filepath.Join(dir, "acs-verdict.json"), acsBody)
	mustWriteRoundArtifact(t, filepath.Join(dir, "audit-report.md"), reportBody)

	retireSupersededAuditArtifacts(dir, 1)

	for _, gone := range []string{"acs-verdict.json", "audit-report.md"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s must be retired from its canonical path (stale round verdict is the cycle-1603 class); stat err=%v", gone, err)
		}
	}
	if got := mustReadRoundArtifact(t, filepath.Join(dir, "acs-verdict.round1.json")); got != acsBody {
		t.Errorf("acs-verdict.round1.json = %q, want the round-1 body preserved", got)
	}
	if got := mustReadRoundArtifact(t, filepath.Join(dir, "audit-report.round1.md")); got != reportBody {
		t.Errorf("audit-report.round1.md = %q, want the round-1 body preserved", got)
	}
}

func TestRetireSupersededAuditArtifacts_RoundZeroHonorsPreStagedVerdict(t *testing.T) {
	// First dispatch: an operator/CI pre-staged acs-verdict.json keeps the
	// honor audit.Classify grants it — nothing is superseded yet.
	dir := t.TempDir()
	mustWriteRoundArtifact(t, filepath.Join(dir, "acs-verdict.json"), "operator-pre-stage")

	retireSupersededAuditArtifacts(dir, 0)

	if got := mustReadRoundArtifact(t, filepath.Join(dir, "acs-verdict.json")); got != "operator-pre-stage" {
		t.Errorf("round 0 must not retire a pre-staged verdict; got %q", got)
	}
}

func TestRetireSupersededAuditArtifacts_NeverClobbersAnExistingArchive(t *testing.T) {
	// An errored attempt re-dispatched within the SAME round: the first
	// retirement already archived the round's true evidence; a partial file
	// written by the dead attempt is dropped, not archived over it.
	dir := t.TempDir()
	mustWriteRoundArtifact(t, filepath.Join(dir, "acs-verdict.round1.json"), "true-round1-evidence")
	mustWriteRoundArtifact(t, filepath.Join(dir, "acs-verdict.json"), "partial-from-dead-attempt")

	retireSupersededAuditArtifacts(dir, 1)

	if got := mustReadRoundArtifact(t, filepath.Join(dir, "acs-verdict.round1.json")); got != "true-round1-evidence" {
		t.Errorf("existing archive clobbered: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "acs-verdict.json")); !os.IsNotExist(err) {
		t.Errorf("stale canonical copy must still be retired; stat err=%v", err)
	}
}

func TestRetireSupersededAuditArtifacts_MissingFilesAndWorkspaceAreQuiet(t *testing.T) {
	retireSupersededAuditArtifacts("", 1)          // no workspace: state-only harnesses
	retireSupersededAuditArtifacts(t.TempDir(), 1) // absent files: fresh shape
}

func TestCompletedAuditRounds_CountsOnlyAuditOccurrences(t *testing.T) {
	tests := []struct {
		name      string
		completed []string
		want      int
	}{
		{"no audit yet", []string{"scout", "tdd", "build"}, 0},
		{"one round", []string{"scout", "tdd", "build", "audit"}, 1},
		{"repair re-entry", []string{"scout", "tdd", "build", "audit", "tdd", "build", "audit"}, 2},
		{"empty", nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := completedAuditRounds(tc.completed); got != tc.want {
				t.Errorf("completedAuditRounds(%v) = %d, want %d", tc.completed, got, tc.want)
			}
		})
	}
}

// THE LIVE PATH — the cycle-1603 shape driven through a real repair cycle. The
// round-1 auditor pre-writes acs-verdict.json (the persona instructs exactly
// that) and FAILs with a repairable class; the granted repair re-enters
// tdd/build; at the round-2 audit DISPATCH the stale verdict must already be
// retired — asserted from inside the runner, at the exact moment the real
// verdict-exists gate would have read it.
func TestAuditRedispatch_RetiresPreviousRoundVerdicts(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &verdictStagingAuditRunner{t: t}
	runners[PhaseAudit] = ar
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if ar.runs < 2 {
		t.Fatalf("audit ran %d time(s); the repair never re-audited (phases=%v)", ar.runs, res.PhasesRun)
	}
	if !ar.round2SawRetirement {
		t.Error("round-2 audit dispatched with round-1's acs-verdict.json still canonical — the cycle-1603 replay is live")
	}
	if got := mustReadRoundArtifact(t, filepath.Join(ar.workspace, "acs-verdict.round1.json")); got != round1VerdictBody {
		t.Errorf("round-1 verdict not archived for forensics; got %q", got)
	}
	if _, err := os.Stat(filepath.Join(ar.workspace, "audit-report.round1.md")); err != nil {
		t.Errorf("round-1 report not archived: %v", err)
	}
}

const round1VerdictBody = `{"verdict":"PASS","red_count":0,"ship_eligible":false}`

// verdictStagingAuditRunner reproduces the live auditor's artifact behavior:
// round 1 pre-writes the acs verdict + a FAIL report with a repairable class;
// round 2 checks the canonical paths were retired before it started and PASSes.
type verdictStagingAuditRunner struct {
	t                   *testing.T
	runs                int
	workspace           string
	round2SawRetirement bool
}

func (r *verdictStagingAuditRunner) Name() string { return string(PhaseAudit) }

func (r *verdictStagingAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.runs++
	r.workspace = req.Workspace
	if r.runs == 1 {
		mustWriteRoundArtifact(r.t, filepath.Join(req.Workspace, "acs-verdict.json"), round1VerdictBody)
		writeAuditWithFailure(r.t, req.Workspace, "FAIL", "code-audit-fail", "H1 substantive defect")
		return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
	}
	_, acsErr := os.Stat(filepath.Join(req.Workspace, "acs-verdict.json"))
	_, repErr := os.Stat(filepath.Join(req.Workspace, "audit-report.md"))
	r.round2SawRetirement = os.IsNotExist(acsErr) && os.IsNotExist(repErr)
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func mustWriteRoundArtifact(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadRoundArtifact(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Resume-surface parity: a cycle that crashed and resumed AT audit must retire
// superseded verdicts on its re-audits exactly like the live loop — the resume
// loop has its own dispatch seam (resume.go), and a miss there would revive
// the cycle-1603 replay on precisely the surface that exists for recovery.
func TestResumedAuditRedispatch_RetiresPreviousRoundVerdicts(t *testing.T) {
	// The resume surface takes its workspace from the PERSISTED cycle-state
	// (authoritativeResumeIdentity), so the harness seeds one — an empty
	// workspace would silently no-op the retirement under test.
	ws := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 1603, WorkspacePath: ws}}
	led := &fakeLedger{}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &verdictStagingAuditRunner{t: t}
	runners[PhaseAudit] = ar
	o := NewOrchestrator(st, led, runners)

	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 1603})
	if err != nil {
		t.Fatalf("resume cycle: %v", err)
	}
	if ar.runs < 2 {
		t.Fatalf("audit ran %d time(s) on resume; the repair never re-audited (phases=%v)", ar.runs, res.PhasesRun)
	}
	if !ar.round2SawRetirement {
		t.Error("resumed round-2 audit dispatched with round-1's verdict still canonical — resume.go's seam is unwired")
	}
}

// The primitive's counter math IS the crash-correctness (review-2 HIGH): each
// call retires by the persisted dispatch count and then advances it, so a
// dispatch that never completes still marks its round superseded. A dropped
// increment would leave every later dispatch at round 0 — permanently
// honoring stale verdicts.
func TestSupersedePreviousAuditRound_AdvancesThePersistedDispatchCounter(t *testing.T) {
	ws := t.TempDir()
	cs := CycleState{WorkspacePath: ws}

	supersedePreviousAuditRound(&cs) // first dispatch: nothing to retire
	if cs.AuditDispatches != 1 {
		t.Fatalf("AuditDispatches = %d after first dispatch, want 1", cs.AuditDispatches)
	}
	mustWriteRoundArtifact(t, filepath.Join(ws, "acs-verdict.json"), "attempt-1")

	supersedePreviousAuditRound(&cs) // re-dispatch: attempt 1 is superseded
	if cs.AuditDispatches != 2 {
		t.Fatalf("AuditDispatches = %d after second dispatch, want 2", cs.AuditDispatches)
	}
	if got := mustReadRoundArtifact(t, filepath.Join(ws, "acs-verdict.round1.json")); got != "attempt-1" {
		t.Errorf("first attempt's verdict not retired as round1; got %q", got)
	}
}

// Legacy backstop: a checkpoint persisted before AuditDispatches existed
// decodes the counter as 0, but its CompletedPhases still names the finished
// rounds — the completion-derived count must floor the index so a mid-repair
// legacy resume still retires.
func TestSupersedePreviousAuditRound_LegacyCheckpointFallsBackToCompletions(t *testing.T) {
	ws := t.TempDir()
	mustWriteRoundArtifact(t, filepath.Join(ws, "acs-verdict.json"), "legacy-round-1")
	cs := CycleState{WorkspacePath: ws, CompletedPhases: []string{"tdd", "build", "audit", "tdd", "build"}}

	supersedePreviousAuditRound(&cs)

	if got := mustReadRoundArtifact(t, filepath.Join(ws, "acs-verdict.round1.json")); got != "legacy-round-1" {
		t.Errorf("legacy checkpoint's completed round not retired; got %q", got)
	}
	if cs.AuditDispatches != 2 {
		t.Errorf("AuditDispatches = %d, want 2 (completion floor + this dispatch)", cs.AuditDispatches)
	}
}

// Review-2 HIGH, the live crash shape: an audit quota-paused/crashed
// MID-FLIGHT after the auditor pre-wrote acs-verdict.json has no
// CompletedPhases entry — only the persisted AuditDispatches counter knows the
// round existed. The resumed re-dispatch must retire the dead attempt's
// verdict instead of honoring it through the verdict-exists gate.
func TestResumedCrashedAudit_RetiresTheDeadAttemptsVerdict(t *testing.T) {
	ws := t.TempDir()
	mustWriteRoundArtifact(t, filepath.Join(ws, "acs-verdict.json"), `{"verdict":"PASS","ship_eligible":false}`)
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{
		CycleID:         1603,
		WorkspacePath:   ws,
		Phase:           string(PhaseAudit),
		AuditDispatches: 1, // the crashed round persisted its dispatch pre-flight
	}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	probe := &retirementProbeRunner{}
	runners[PhaseAudit] = probe
	o := NewOrchestrator(st, led, runners)

	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 1603}); err != nil {
		t.Fatalf("resume cycle: %v", err)
	}
	if !probe.sawRetirement {
		t.Error("resumed audit dispatched with the dead attempt's acs-verdict.json still canonical — the crashed-round replay is live")
	}
	if got := mustReadRoundArtifact(t, filepath.Join(ws, "acs-verdict.round1.json")); got != `{"verdict":"PASS","ship_eligible":false}` {
		t.Errorf("dead attempt's verdict not archived; got %q", got)
	}
}

// retirementProbeRunner records, at its first dispatch, whether the canonical
// acs-verdict.json had been retired — the exact read the real verdict-exists
// gate performs — then PASSes.
type retirementProbeRunner struct{ sawRetirement bool }

func (r *retirementProbeRunner) Name() string { return string(PhaseAudit) }

func (r *retirementProbeRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	_, err := os.Stat(filepath.Join(req.Workspace, "acs-verdict.json"))
	r.sawRetirement = os.IsNotExist(err)
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}
