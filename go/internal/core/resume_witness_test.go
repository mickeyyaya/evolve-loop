package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

const witnessCycle = 7

func resumedAfterShipFixture(t *testing.T, root string) (*Orchestrator, *fakeStorage, CycleRequest, *ResumePoint) {
	t.Helper()
	ws := RunWorkspacePath(root, witnessCycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, ws, "ship-error.json", transientShipError)
	st := &fakeStorage{state: State{LastCycleNumber: witnessCycle}, cycleState: CycleState{
		CycleID:         witnessCycle,
		RunID:           "host-run",
		WorkspacePath:   ws,
		FinalVerdict:    VerdictPASS,
		CompletedPhases: []string{"scout", "triage", "build", "audit", "ship"},
	}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	return o, st, CycleRequest{ProjectRoot: root, GoalHash: "recover-original-task"}, &ResumePoint{Phase: string(PhaseRetro), CycleID: witnessCycle}
}

func hostWitness(t *testing.T, root string, cs CycleState) {
	t.Helper()
	if err := recordResumeWitness(root, cs); err != nil {
		t.Fatalf("record host witness: %v", err)
	}
}

func resumeLanding(t *testing.T, o *Orchestrator, req CycleRequest, rp *ResumePoint) CycleResult {
	t.Helper()
	got, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestResumeWitness_AShipLatchPlantedInRunStateIsStillALostLanding(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		hostShipped bool
		wantLost    bool
	}{
		{"the host never latched the ship", false, true},
		{"the host latched the ship", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _, _ := landedRepo(t)
			o, st, req, rp := resumedAfterShipFixture(t, root)
			hostWitness(t, root, CycleState{CycleID: witnessCycle, RunID: "host-run", Shipped: tc.hostShipped, PreCycleHEAD: gitRev(t, root, "HEAD")})
			st.cycleState.Shipped = true
			got := resumeLanding(t, o, req, rp)
			lost := got.SystemFailure != nil && got.SystemFailure.Category == "landing-lost"
			if lost != tc.wantLost {
				t.Fatalf("lost=%v, want %v: the resume must take the ship latch from the host witness, not run.json: %+v", lost, tc.wantLost, got)
			}
		})
	}
}

func TestResumeWitness_APreCycleHeadMovedBackInRunStateDoesNotClearTheFloor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		hostPre  string
		wantLost bool
	}{
		{"the host pre-cycle HEAD already holds the named commit", "HEAD", true},
		{"the host pre-cycle HEAD is before the named commit", "HEAD^", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, commit, tree := landedRepo(t)
			o, st, req, rp := resumedAfterShipFixture(t, root)
			plantBinding(t, st.cycleState.WorkspacePath, dossier.ShipBinding{CommitSHA: commit, TreeSHACommitted: tree, Cycle: witnessCycle})
			hostWitness(t, root, CycleState{CycleID: witnessCycle, RunID: "host-run", PreCycleHEAD: gitRev(t, root, tc.hostPre)})
			st.cycleState.PreCycleHEAD = gitRev(t, root, "HEAD^")
			got := resumeLanding(t, o, req, rp)
			lost := got.SystemFailure != nil && got.SystemFailure.Category == "landing-lost"
			if lost != tc.wantLost {
				t.Fatalf("lost=%v, want %v: the resume must bound the binding by the host pre-cycle HEAD, not run.json: %+v", lost, tc.wantLost, got)
			}
		})
	}
}

func TestResumeWitness_AWitnessThatDoesNotNameThisRunIsNotTrusted(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		witness string
	}{
		{"no witness", ""},
		{"a witness of another run", `{"cycle":7,"run_id":"other-run","shipped":true,"pre_cycle_head":"abc"}`},
		{"a witness of another cycle", `{"cycle":8,"run_id":"host-run","shipped":true,"pre_cycle_head":"abc"}`},
		{"a corrupt witness", `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tc.witness != "" {
				path := resumeWitnessPath(root, witnessCycle)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.witness), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cs := CycleState{CycleID: witnessCycle, RunID: "host-run", Shipped: true, PreCycleHEAD: "forged"}
			applyResumeWitness(root, witnessCycle, &cs)
			if cs.Shipped || cs.PreCycleHEAD != "" {
				t.Fatalf("an untrusted witness kept the run.json values: shipped=%v pre=%q", cs.Shipped, cs.PreCycleHEAD)
			}
		})
	}
}

func TestResumeWitness_TheHostRecordsTheWitnessWhereItSetsTheFields(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		shipVerdict string
		wantShipped bool
	}{
		{"a fresh cycle whose ship passed", VerdictPASS, true},
		{"a fresh cycle whose ship failed", VerdictFAIL, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := initDossierRepo(t)
			st := &fakeStorage{state: State{LastCycleNumber: 4}}
			o := NewOrchestrator(st, &fakeLedger{}, buildRunners(map[Phase]string{PhaseShip: tc.shipVerdict}))
			o.gitHEAD = func() (string, error) { return "pre-cycle-sha", nil }
			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "goal"})
			if err != nil && tc.wantShipped {
				t.Fatal(err)
			}
			cs := CycleState{CycleID: res.Cycle, RunID: st.cycleState.RunID}
			applyResumeWitness(root, res.Cycle, &cs)
			if cs.Shipped != tc.wantShipped || cs.PreCycleHEAD != "pre-cycle-sha" {
				t.Fatalf("the host witness holds shipped=%v pre=%q, want shipped=%v pre=%q", cs.Shipped, cs.PreCycleHEAD, tc.wantShipped, "pre-cycle-sha")
			}
		})
	}
}

func TestResumeWitness_AResumedShipPassRecordsTheLatch(t *testing.T) {
	t.Parallel()

	o, st, req, rp := resumedAfterShipFixture(t, initDossierRepo(t))
	st.cycleState.CompletedPhases = []string{"scout", "triage", "build"}
	rp.Phase = string(PhaseAudit)
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{CycleID: 7, RunID: st.cycleState.RunID}
	applyResumeWitness(req.ProjectRoot, 7, &cs)
	if !cs.Shipped {
		t.Fatal("a ship that passed on the resume root did not record the host latch")
	}
}

func TestResumeWitness_AnUnwritableWitnessDirIsAnError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := recordResumeWitness(root, CycleState{CycleID: witnessCycle}); err == nil {
		t.Fatal("a witness the host cannot write must be reported")
	}
	var stderr bytes.Buffer
	o := &Orchestrator{stderr: &stderr}
	o.persistResumeWitness(root, CycleState{CycleID: witnessCycle})
	if !strings.Contains(stderr.String(), "resume witness not recorded") {
		t.Fatalf("an unrecorded witness must be logged, got %q", stderr.String())
	}
}

func TestResumeWitness_AnOverwrittenShipErrorStillFlagsTheLostLanding(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		hostSaw  bool
		wantLost bool
	}{
		{"the host recorded the ship error", true, true},
		{"the host recorded no ship error", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ws := RunWorkspacePath(root, witnessCycle)
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			cs := CycleState{CycleID: witnessCycle, RunID: "host-run", WorkspacePath: ws}
			if tc.hostSaw {
				o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&bytes.Buffer{}))
				o.recoverFromShipError(context.Background(), root, witnessCycle, &cs,
					NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"), 99, 1)
			}
			writeWorkspaceFile(t, ws, "ship-error.json", `{}`)
			sig := detectLostLanding(landingEvidence{Workspace: ws, ProjectRoot: root, Cycle: witnessCycle, FinalVerdict: VerdictPASS})
			if lost := sig != nil && sig.Category == "landing-lost"; lost != tc.wantLost {
				t.Fatalf("lost=%v, want %v: the floor must read the ship error the host recorded, not the agent-writable file", lost, tc.wantLost)
			}
		})
	}
}

func TestResumeWitness_ALatchAfterAShipErrorKeepsTheShipError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cs := CycleState{CycleID: witnessCycle, RunID: "host-run", PreCycleHEAD: "pre"}
	o := &Orchestrator{stderr: &bytes.Buffer{}}
	o.persistShipErrorWitness(root, cs, NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "peer moved main"))
	if !o.latchShip(root, &cs, PhaseShip, VerdictPASS) {
		t.Fatal("a ship PASS must latch")
	}
	w, ok := readResumeWitness(root, witnessCycle)
	if !ok || !w.Shipped || w.ShipError == nil || w.ShipError.Code != string(CodeGitFleetRebaseNeeded) || w.PreCycleHEAD != "pre" {
		t.Fatalf("the latch must keep the earlier ship error and the pre-cycle HEAD: %+v ok=%v", w, ok)
	}
	next := CycleState{CycleID: witnessCycle, RunID: "next-run"}
	if err := recordResumeWitness(root, next); err != nil {
		t.Fatal(err)
	}
	if w, _ := readResumeWitness(root, witnessCycle); w.ShipError != nil {
		t.Fatalf("a ship error of another run must not carry into this run's witness: %+v", w)
	}
}

func TestResumeWitness_AnUnrecordedShipErrorIsLogged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	o := &Orchestrator{stderr: &stderr}
	o.persistShipErrorWitness(root, CycleState{CycleID: witnessCycle}, NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "x"))
	if !strings.Contains(stderr.String(), "ship error not recorded in the resume witness") {
		t.Fatalf("an unrecorded ship error must be logged, got %q", stderr.String())
	}
}

func TestResumeWitness_TheResumeTakesTheWitnessOfTheHostCycle(t *testing.T) {
	t.Parallel()

	root, _, _ := landedRepo(t)
	o, st, req, rp := resumedAfterShipFixture(t, root)
	hostWitness(t, root, CycleState{CycleID: witnessCycle - 1, RunID: "host-run", Shipped: true, PreCycleHEAD: gitRev(t, root, "HEAD")})
	st.cycleState.CycleID = witnessCycle - 1
	rp.CycleID = 0
	got, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatal(err)
	}
	if got.SystemFailure == nil || got.SystemFailure.Category != "landing-lost" {
		t.Fatalf("a state file naming cycle %d borrowed that cycle's shipped witness for host cycle %d: %+v", witnessCycle-1, witnessCycle, got)
	}
}
