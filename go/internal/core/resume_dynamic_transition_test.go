package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const insertedResumePhase = Phase("bug-reproduction")

// writeRoutingPlan writes a whole-cycle plan to <workspace>/routing-plan.json
// in the same on-disk shape parsePhasePlan reads: a bare JSON array of
// {"phase","run"} entries.
func writeRoutingPlan(t *testing.T, workspace string, entries []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "routing-plan.json"), raw, 0o644); err != nil {
		t.Fatalf("write routing-plan.json: %v", err)
	}
}

// fullRunnerSet is buildRunners plus the two spine runners it omits
// (PhaseSwarmPlan, PhaseDebugger) so that whichever static successor a degrade
// path selects always has a registered runner — isolating the transition-kernel
// behavior under test from an incidental missing-runner abort.
func fullRunnerSet() map[Phase]PhaseRunner {
	runners := buildRunners(nil)
	runners[PhaseSwarmPlan] = &fakeRunner{name: string(PhaseSwarmPlan)}
	runners[PhaseDebugger] = &fakeRunner{name: string(PhaseDebugger)}
	return runners
}

func TestRunCycleFromPhase_InsertedPhaseTransitionsViaPlan(t *testing.T) {
	t.Parallel()
	if insertedResumePhase.IsValid() {
		t.Fatalf("test premise broken: %q must NOT be a spine-valid phase", insertedResumePhase)
	}
	ws := t.TempDir()
	writeRoutingPlan(t, ws, []map[string]any{
		{"phase": string(insertedResumePhase), "run": true},
		{"phase": string(PhaseAudit), "run": true},
		{"phase": string(PhaseShip), "run": true},
	})

	st := &fakeStorage{
		state:      State{LastCycleNumber: 635},
		cycleState: CycleState{CycleID: 635, WorkspacePath: ws},
	}
	runners := fullRunnerSet()
	runners[insertedResumePhase] = &fakeRunner{name: string(insertedResumePhase)}
	auditRunner := runners[PhaseAudit].(*fakeRunner)
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(insertedResumePhase), CycleID: 635})

	if err != nil && strings.Contains(err.Error(), "invalid phase") {
		t.Fatalf("RED: resume died transitioning out of inserted phase %q: %v", insertedResumePhase, err)
	}
	if auditRunner.calls == 0 {
		t.Fatalf("RED: transition out of inserted phase %q did not reach the next planned phase (audit); err=%v",
			insertedResumePhase, err)
	}
	if err != nil {
		t.Fatalf("resume with a valid plan must complete cleanly, got: %v", err)
	}
}

func TestRunCycleFromPhase_MissingPlanDegradesNotInvalidPhase(t *testing.T) {
	t.Parallel()
	if insertedResumePhase.IsValid() {
		t.Fatalf("test premise broken: %q must NOT be a spine-valid phase", insertedResumePhase)
	}
	ws := t.TempDir() // deliberately no routing-plan.json written here

	st := &fakeStorage{
		state:      State{LastCycleNumber: 635},
		cycleState: CycleState{CycleID: 635, WorkspacePath: ws},
	}
	runners := fullRunnerSet()
	inserted := &fakeRunner{name: string(insertedResumePhase)}
	runners[insertedResumePhase] = inserted
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(insertedResumePhase), CycleID: 635})

	if err != nil && strings.Contains(err.Error(), "invalid phase") {
		t.Fatalf("RED: missing plan must degrade to an archetype transition, not invalid-phase: %v", err)
	}
	if inserted.calls == 0 {
		t.Fatalf("inserted phase %q was never dispatched on resume", insertedResumePhase)
	}
	if err != nil {
		t.Fatalf("degraded resume must proceed without error, got: %v", err)
	}
}

func TestRunCycleFromPhase_TransitionFailureRecordsAbortReason(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	st := &fakeStorage{
		state:      State{LastCycleNumber: 635},
		cycleState: CycleState{CycleID: 635, WorkspacePath: ws},
	}
	runners := buildRunners(nil)
	delete(runners, PhaseAudit)
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseBuild), CycleID: 635})
	if err == nil {
		t.Fatalf("test premise: stranded-successor resume must still surface an error")
	}

	sidecar := filepath.Join(ws, string(PhaseAudit)+"-usage.json")
	raw, readErr := os.ReadFile(sidecar)
	if readErr != nil {
		t.Fatalf("RED: resume transition failure escaped the C1 chokepoint — no %s-usage.json recorded (FAILED_UNEXPLAINED); err=%v",
			PhaseAudit, err)
	}
	var rec struct {
		AbortReason string `json:"abort_reason"`
	}
	if uErr := json.Unmarshal(raw, &rec); uErr != nil {
		t.Fatalf("usage sidecar is not valid JSON: %v", uErr)
	}
	if strings.TrimSpace(rec.AbortReason) == "" {
		t.Fatalf("RED: recorded outcome for the stalled phase carries no abort_reason (still FAILED_UNEXPLAINED)")
	}
}
