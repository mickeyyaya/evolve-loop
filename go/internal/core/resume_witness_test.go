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
