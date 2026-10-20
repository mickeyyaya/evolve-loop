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
	CodeLaneLaunched              signalcenter.Code = "LOOP_ORCH_LANE_LAUNCHED"
	CodeLaneExited                signalcenter.Code = "LOOP_ORCH_LANE_EXITED"
	CodeLaneBackfilled            signalcenter.Code = "LOOP_ORCH_LANE_BACKFILLED"
	CodePoolIdle                  signalcenter.Code = "LOOP_ORCH_POOL_IDLE"
	CodePoolResupplied            signalcenter.Code = "LOOP_ORCH_POOL_RESUPPLIED"
	CodeLaneStalled               signalcenter.Code = "LOOP_ORCH_LANE_STALLED"
	CodeLaneDeadline              signalcenter.Code = "LOOP_ORCH_LANE_DEADLINE"
	CodeLaneLateExit              signalcenter.Code = "LOOP_ORCH_LANE_LATE_EXIT"
	CodeLaneRecoveryFailed        signalcenter.Code = "LOOP_ORCH_LANE_RECOVERY_FAILED"
	CodeOversightAction           signalcenter.Code = "LOOP_ORCH_OVERSIGHT_ACTION"
	CodeFamilyPaused              signalcenter.Code = "LOOP_ORCH_FAMILY_PAUSED"
	CodeSupply                    signalcenter.Code = "LOOP_ORCH_SUPPLY"
	CodeSupplyShort               signalcenter.Code = "LOOP_ORCH_SUPPLY_SHORT"
	CodeWindow                    signalcenter.Code = "LOOP_ORCH_WINDOW"
	CodePoolUnderfilled           signalcenter.Code = "LOOP_ORCH_POOL_UNDERFILLED"
	CodeAdmissionShadow           signalcenter.Code = "LOOP_ORCH_ADMISSION_SHADOW"
	CodeAdmissionOptimistic       signalcenter.Code = "LOOP_ORCH_ADMISSION_OPTIMISTIC"
	CodeOptimisticAdmissionFailed signalcenter.Code = "LOOP_ORCH_OPTIMISTIC_ADMISSION_FAILED"
)

const (
	CodePhaseStalled  signalcenter.Code = "LIVENESS_PHASE_STALLED"
	CodeUsageEvidence signalcenter.Code = "BRIDGE_USAGE_EVIDENCE"
	CodePaneExhausted signalcenter.Code = "LIVENESS_PANE_EXHAUSTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLaunched, "the orchestrator pool (fleet.orchestrator.stage enforce, ADR-0139 O4) launched a lane in its first fill; INFO; fields.todo, live (lanes in flight), target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneExited, "a lane of the orchestrator pool exited and freed its slot and its cells; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneBackfilled, "the orchestrator pool gave a freed slot to the next admissible todo at once; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolIdle, "a slot of the orchestrator pool stays free after a lane exit: no pending todo is admissible, the supply is empty, the operator stop is engaged or the run reached fleet.orchestrator.run_max_lanes; INFO; fields.live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolResupplied, "a slot of the orchestrator pool stayed free after a lane exit, so the pool ran the plan source again and added a ready todo that the run did not hold; one event for each added todo; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneStalled, "a lane had no live signal (its cycle signals, its phase pane-watch, events or stdout file) for fleet.orchestrator.stall_minutes (ADR-0139 O5); published once for each lane; a lane whose cycle is not found has an unknown heartbeat and no stall; the lane keeps its slot; fields.todo, stage, last_heartbeat, stall_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneDeadline, "a lane passed its lease deadline (the p95 lane time of the last 50 cycles without the longest one, at least fleet.orchestrator.deadline_floor_minutes, ADR-0139 O5); action recorded at stage shadow (nothing stops), action stopped at stage enforce (the lane's own launch context is cancelled, its dispatches are reaped, its inbox claim is released and its cells free); fields.todo, stage, action, deadline_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLateExit, "a lane exited after its lease deadline had passed; the shadow exit measure of ADR-0139 O5: status ok means a deadline stop would have stopped a lane that passed; fields.todo, stage, status, rc, stopped")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneRecoveryFailed, "after a deadline stop the orchestrator could not release the todo's inbox claim (step release) or park it with route console (step park, reason deadline twice); the todo needs the console; fields.todo, step, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOversightAction, "the oversight loop (ADR-0139 O6) mapped one event of the cycle, ship, errors or loop channel to an action on its own lanes: record_verdict, free_cells, count_clean_landing, count_conflict, backfill, resize or stop_admission; at stage shadow the action is recorded and changes nothing (applied false), at stage enforce it is applied (applied true); INFO; fields.actions, lane, cycle, event, stage, applied, and verdict or target for the actions that have one")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeFamilyPaused, "a bench left a CLI family with no healthy family after it in some lane chain, so the sizing step costs a slot and the family stops taking new lanes (ADR-0139 decision 13); lanes in flight finish; published once for each pause; at stage shadow it records the pause and changes nothing (applied false); INFO; fields.family, target (the window after the pause), stage, applied")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeSupply, "the supply census (ADR-0139 O6, spec section 4) before a plan: the count of the inbox items in each class (admissible, waiting_cell, waiting_deps, console_route, console_kind, console_protected), the window, and the first console items with the reason of the routing authority; the full record is .evolve/waves/supply.json; INFO; fields are the six class names, window and console")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeSupplyShort, "the supply census found fewer admissible items than the window, so the lanes of the plan cannot all be filled; the cause of each refusal is in the supply record and in evolve wave supply; INFO; fields are the six class names and window")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWindow, "the adaptive window of the orchestrator pool (ADR-0139 O8) changed size: it grows by 1 for each clean landing, halves on each landing conflict (never under fleet.orchestrator.window.min) and follows the sized count; published once for each change; INFO; fields.old, new, sized, cause (clean_landing, conflict or resize); the loop builds the window inside the enforce pool only, so nothing publishes at stage shadow; that pool admits a lane only while the lanes in flight number fewer than new")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolUnderfilled, "the orchestrator pool had an idle transition while fewer lanes than fleet.min_lanes were in flight and the pool was not stopped; WARN; fields.live, target, min_lanes, ready, and family_held when admission holds a todo behind a lane of its id family (each \"<todo> (family:<token>, behind <lane>)\"), so the hold does not read as a supply failure")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeAdmissionShadow, "at fleet.orchestrator.admission shadow (ADR-0143 W1) the import-reach rule held a todo that the optimistic rule (a shared edited package or a second hub todo) would admit; the reach rule still governs; published once for each todo and holder in a pool run; INFO; fields.todo, held_by, rank, cells, reason (the package where an edit meets the reach)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeAdmissionOptimistic, "at fleet.orchestrator.admission enforce (ADR-0143 W1b) the pool launched a lane that only the optimistic rule admitted: the import-reach rule would have held it behind a lane in flight; held_by is the first such holder when more than one lane in flight reaches the todo; the lane carries that holder in EVOLVE_FLEET_REACH_HOLDER; the count of the wave; INFO; fields.todo, held_by, reason (the package where an edit meets the reach)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOptimisticAdmissionFailed, "a lane that only the optimistic rule admitted (ADR-0143 W1b) hit a composed-tree gate failure or a fleet rebase conflict; published once for each lane, at its first such failure, which names the gate; the D3 rate counts a failure only when the admission of the same todo is in the wave, so it stays in [0,1]; WARN; fields.todo, held_by, gate (the failed composed-tree gates, or rebase)")
}

type Publish func(signalcenter.Event)

func (p Publish) emit(origin string, code signalcenter.Code, severity signalcenter.Severity, reason string, fields map[string]string) {
	if p == nil {
		return
	}
	p(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopWave,
		Severity: severity, Code: code, Reason: reason, Fields: maps.Clone(fields)})
}
