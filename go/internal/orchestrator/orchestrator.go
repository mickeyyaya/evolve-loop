// Package orchestrator runs the lanes of the loop: a rolling pool that admits by cell and a lease that stops a lane past its deadline.
// See docs/architecture/packages/internal-orchestrator.md.
package orchestrator

import (
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	StageShadow  = "shadow"
	StageEnforce = "enforce"
)

const (
	CodeLaneLaunched       signalcenter.Code = "LOOP_ORCH_LANE_LAUNCHED"
	CodeLaneExited         signalcenter.Code = "LOOP_ORCH_LANE_EXITED"
	CodeLaneBackfilled     signalcenter.Code = "LOOP_ORCH_LANE_BACKFILLED"
	CodePoolIdle           signalcenter.Code = "LOOP_ORCH_POOL_IDLE"
	CodeLaneStalled        signalcenter.Code = "LOOP_ORCH_LANE_STALLED"
	CodeLaneDeadline       signalcenter.Code = "LOOP_ORCH_LANE_DEADLINE"
	CodeLaneLateExit       signalcenter.Code = "LOOP_ORCH_LANE_LATE_EXIT"
	CodeLaneRecoveryFailed signalcenter.Code = "LOOP_ORCH_LANE_RECOVERY_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLaunched, "the orchestrator pool (fleet.orchestrator.stage enforce, ADR-0139 O4) launched a lane in its first fill; INFO; fields.todo, live (lanes in flight), target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneExited, "a lane of the orchestrator pool exited and freed its slot and its cells; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneBackfilled, "the orchestrator pool gave a freed slot to the next admissible todo at once; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolIdle, "a slot of the orchestrator pool stays free after a lane exit: no pending todo is admissible, the supply is empty, the operator stop is engaged or the run reached fleet.orchestrator.run_max_lanes; INFO; fields.live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneStalled, "a lane had no live signal (its cycle signals, its phase pane-watch, events or stdout file) for fleet.orchestrator.stall_minutes (ADR-0139 O5); published once for each lane; a lane whose cycle is not found has an unknown heartbeat and no stall; the lane keeps its slot; fields.todo, stage, last_heartbeat, stall_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneDeadline, "a lane passed its lease deadline (the p95 lane time of the last 50 cycles without the longest one, at least fleet.orchestrator.deadline_floor_minutes, ADR-0139 O5); action recorded at stage shadow (nothing stops), action stopped at stage enforce (the lane's own launch context is cancelled, its dispatches are reaped, its inbox claim is released and its cells free); fields.todo, stage, action, deadline_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLateExit, "a lane exited after its lease deadline had passed; the shadow exit measure of ADR-0139 O5: status ok means a deadline stop would have stopped a lane that passed; fields.todo, stage, status, rc, stopped")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneRecoveryFailed, "after a deadline stop the orchestrator could not release the todo's inbox claim (step release) or park it with route console (step park, reason deadline twice); the todo needs the console; fields.todo, step, error")
}

type Publish func(signalcenter.Event)

func (p Publish) emit(origin string, code signalcenter.Code, severity signalcenter.Severity, reason string, fields map[string]string) {
	if p == nil {
		return
	}
	p(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopWave,
		Severity: severity, Code: code, Reason: reason, Fields: maps.Clone(fields)})
}
