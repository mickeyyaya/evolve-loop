package core

// resume_dynamic_transition_test.go — RED regression suite for cycle-637 task
// resume-dynamic-phase-transition (inbox 2026-07-10T01-30-00Z, weight 0.93).
//
// Defect: `evolve loop --resume` replays the checkpointed advisor-inserted phase
// (e.g. bug-reproduction, fault-localization — real catalog phases the dynamic
// router splices onto bugfix cycles), the phase re-runs and PASSes, but the
// transition kernel consulted AFTER it (o.sm.Next in RunCycleFromPhase's dispatch
// loop) only knows the static spine. current.IsValid()==false for the inserted
// phase, so Next returns "core: invalid phase: <phase>", the resumed cycle dies,
// and — because that transition-error return escapes the ADR-0044 C1 recording
// chokepoint — it is paged FAILED_UNEXPLAINED with no abort_reason.
//
// Evidence: 2026-07-10 resume of cycle 635 (resumeFromPhase=bug-reproduction):
// "evolve loop: resume cycle 635: transition from bug-reproduction: core: invalid
// phase: bug-reproduction"; outcome FAILED_UNEXPLAINED "a terminal path escaped
// the C1 chokepoint".
//
// These tests are plain (untagged) package-core tests so they run in the default
// `go test ./internal/core/...` suite and under -race. They reuse the in-package
// fakes (fakeStorage/fakeLedger/fakeRunner/buildRunners) from orchestrator_test.go
// exactly as the existing resume coverage tests do. The cycle-637 ACS predicates
// (go/acs/cycle637) shell `go test -run` over each one.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// insertedResumePhase is a real advisor-inserted catalog phase name that is NOT
// one of the spine constants Phase.IsValid() recognizes — the class of phase the
// defect strands on resume.
const insertedResumePhase = Phase("bug-reproduction")

// writeRoutingPlan writes the advisor's whole-cycle plan to
// <workspace>/routing-plan.json in the SAME on-disk shape parsePhasePlan reads:
// a bare JSON array of {"phase","run"} entries. Mirrors the real artifact
// (.evolve/runs/cycle-N/routing-plan.json) the sealed cycle-635 run produced.
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

// TestRunCycleFromPhase_InsertedPhaseTransitionsViaPlan — AC1 (predicate).
// Resuming from an advisor-inserted phase must rehydrate the transition kernel
// from the run's routing-plan.json and transition to the NEXT PLANNED phase,
// NOT die with "core: invalid phase". Behavioral proof: the planned successor's
// runner is actually dispatched.
//
// Plan: [bug-reproduction, audit, ship] — the successor of the inserted phase is
// audit. RED baseline: o.sm.Next(bug-reproduction, PASS) returns invalid-phase,
// so the loop returns before audit ever dispatches (audit.calls==0).
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

// TestRunCycleFromPhase_MissingPlanDegradesNotInvalidPhase — AC2 (predicate,
// negative/degrade axis). When routing-plan.json is absent (deleted, or a
// pre-plan checkpoint) the resume must DEGRADE the inserted phase to its
// archetype and route to the next static phase — NOT surface "core: invalid
// phase". The inserted phase still dispatches; the cycle proceeds without error.
//
// RED baseline: no plan, o.sm.Next(bug-reproduction, PASS) → invalid-phase error.
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

// TestRunCycleFromPhase_TransitionFailureRecordsAbortReason — AC3 (predicate).
// The ADR-0044 C1 invariant on the resume dispatch loop: NO terminal path may
// escape the recording chokepoint. When the cursor cannot continue after a
// transition (here: the resolved successor has no registered runner), the
// failure must funnel through recordPhaseOutcome — writing a <phase>-usage.json
// sidecar carrying a non-empty abort_reason — so the outcome is FAILED_EXPLAINED,
// never the FAILED_UNEXPLAINED the cycle-635 resume produced.
//
// RED baseline: resume.go returns the transition/no-runner error bare, without
// recording an outcome for the stalled phase, so audit-usage.json is never
// written and cyclehealth pages the cycle FAILED_UNEXPLAINED.
func TestRunCycleFromPhase_TransitionFailureRecordsAbortReason(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	st := &fakeStorage{
		state:      State{LastCycleNumber: 635},
		cycleState: CycleState{CycleID: 635, WorkspacePath: ws},
	}
	// Resume from build (spine-valid) but strand its successor: no audit runner.
	// build PASSes, the loop transitions build→audit, and the cursor stalls with
	// no runner for audit — the terminal path that must be recorded, not escape.
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
