// Package orchestrator runs the lanes of the loop: a rolling pool that admits by cell and a lease that stops a lane past its deadline.
// See docs/architecture/packages/internal-orchestrator.md.
package orchestrator

import (
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	StageShadow  = "shadow"
	StageEnforce = "enforce"
)

func IsLaneLifecycle(code signalcenter.Code) bool {
	switch code {
	case CodeLaneLaunched, CodeLaneExited, CodeLaneBackfilled, CodeLaneLateExit:
		return true
	}
	return false
}

const (
	CodeLaneLaunched              signalcenter.Code = "LOOP_ORCH_LANE_LAUNCHED"
	CodeLaneExited                signalcenter.Code = "LOOP_ORCH_LANE_EXITED"
	CodeLaneBackfilled            signalcenter.Code = "LOOP_ORCH_LANE_BACKFILLED"
	CodePoolIdle                  signalcenter.Code = "LOOP_ORCH_POOL_IDLE"
	CodePoolHeld                  signalcenter.Code = "LOOP_ORCH_POOL_HELD"
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
	CodeEjectionBriefFailed       signalcenter.Code = "LOOP_ORCH_EJECTION_BRIEF_FAILED"
	CodeLandingUnlocated          signalcenter.Code = "LOOP_ORCH_LANDING_UNLOCATED"
	CodeHostPressure              signalcenter.Code = "LOOP_ORCH_HOST_PRESSURE"
	CodeHostBlind                 signalcenter.Code = "LOOP_HOST_PRESSURE_BLIND"
	CodeHaltDraining              signalcenter.Code = "LOOP_ORCH_HALT_DRAINING"
)

const (
	CodePhaseStalled       signalcenter.Code = "LIVENESS_PHASE_STALLED"
	CodeUsageEvidence      signalcenter.Code = "BRIDGE_USAGE_EVIDENCE"
	CodeIntegrationEjected                   = integration.CodeEjected
	CodePaneExhausted      signalcenter.Code = "LIVENESS_PANE_EXHAUSTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLaunched, "the orchestrator pool (fleet.orchestrator.stage enforce, ADR-0139 O4) launched a lane in its first fill; INFO; fields.todo, live (lanes in flight), target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneExited, "a lane of the orchestrator pool exited and freed its slot and its cells; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneBackfilled, "the orchestrator pool gave a freed slot to the next admissible todo at once; INFO; fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolIdle, "an orchestrator pool slot stays free after a lane exit: no admissible todo, empty supply, operator stop, or run_max_lanes reached; INFO. Fields: fields.live, target, cause ("+idleCauseList+")")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolResupplied, "an orchestrator pool slot stayed free after a lane exit, so the pool re-ran the plan source and added a ready todo. One event per added todo; INFO. Fields: fields.todo, live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneStalled, "a lane had no live signal (cycle signals, pane-watch, events, or stdout) for fleet.orchestrator.stall_minutes (ADR-0139 O5). Published once per lane; missing cycles have unknown heartbeats and keep their slot. Fields: fields.todo, stage, last_heartbeat, stall_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneDeadline, "a lane passed its lease deadline (p95 of last 50 cycles without max, min deadline_floor_minutes, ADR-0139 O5). Shadow records action; enforce cancels context, reaps dispatches, releases claim, and frees cells. Fields: fields.todo, stage, action, deadline_minutes")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneLateExit, "a lane exited after its lease deadline passed (shadow measure of ADR-0139 O5). status ok means a deadline stop stops a passing lane. Fields: fields.todo, stage, status, rc, stopped")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLaneRecoveryFailed, "after a deadline stop, the orchestrator failed to release the inbox claim (step release) or park with route console (step park). The todo needs the console. Fields: fields.todo, step, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOversightAction, "the oversight loop (ADR-0139 O6) mapped a cycle, ship, errors, or loop event to a lane action. At shadow applied is false; at enforce applied is true; INFO. Fields: fields.actions, lane, cycle, event, stage, applied, verdict/target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeFamilyPaused, "a bench left a CLI family with no healthy fallback in a chain, so sizing drops a slot and stops new lanes (ADR-0139 decision 13). In-flight lanes finish; shadow sets applied false; INFO. Fields: fields.family, target, stage, applied")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeSupply, "the supply census (ADR-0139 O6, spec section 4) before a plan counts inbox items in 7 classes plus window and console items. The full record is .evolve/waves/supply.json; INFO. Fields: seven class names, window, console")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeSupplyShort, "the supply census found fewer admissible items than the window, so the plan lanes cannot all be filled. Refusal causes sit in the supply record and evolve wave supply; INFO. Fields: seven class names, window")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeWindow, "the adaptive orchestrator pool window (ADR-0139 O8) changed size: +1 per clean landing, halved on conflict (min window.min), following sized count. Publishes once per change in enforce pool only; INFO. Fields: fields.old, new, sized, cause")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolHeld, "a pool fill round stopped below its width because admission refused every pending todo while a lane was in flight. Fires once per live/pending count change at wave start and round end; INFO. Fields: fields.live, target")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodePoolUnderfilled, "the pool had an idle or held transition with fewer than fleet.min_lanes in flight while not stopped; WARN. family_held names family-held todos so they do not read as supply failures. Fields: fields.live, target, min_lanes, ready, cause ("+idleCauseList+"), family_held")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeAdmissionShadow, "at fleet.orchestrator.admission shadow (ADR-0143 W1), import-reach held a todo that the optimistic rule admits; reach still governs. Published once per todo and holder in a pool run; INFO. Fields: fields.todo, held_by, rank, cells, reason")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeAdmissionOptimistic, "at fleet.orchestrator.admission enforce (ADR-0143 W1b), the pool launched a lane admitted only by the optimistic rule. held_by names the first reach holder and travels in EVOLVE_FLEET_REACH_HOLDER; INFO. Fields: fields.todo, held_by, reason")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeOptimisticAdmissionFailed, "an optimistically admitted lane (ADR-0143 W1b) hit a composed-tree gate failure or fleet rebase conflict. Published once at first failure; D3 counts failures only when admission of that todo is in-wave; WARN. Fields: fields.todo, held_by, gate")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeEjectionBriefFailed, "oversight (ADR-0147 I3) failed to send the ejection brief (no brief port, unreadable file, or send error). Lane cells stay held, and the lane misses why its deliverable returned; WARN. Fields: fields.lane, cycle, error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLandingUnlocated, "oversight received a landing or seal for a cycle that no live lane holds. It runs landing-state cleanup, counts nothing, and acts on no lane; WARN. fields.cycle, event")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeHostPressure, "the host-pressure controller (fleet.orchestrator.window.adaptive) decided an admission cap from host load, memory and swap. One event per change of the cap; shadow sets applied false and leaves the window alone; INFO. Fields: fields.old, new, sized, cause (pressure_high, pressure_low or hold), load1, cpus, load_per_core, mem_available_pct, swap_used_mb, score, stage, applied")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeHaltDraining, "a lane exited with the ADR-0072 system-failure halt code, so the pool stops its backfill and drains: the running lanes seal and the wave ends. INCIDENT. Fields: fields.todo, rc, cause (system_halt), live")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeHostBlind, "the host-pressure reader failed smooth_samples times in a row, so the controller holds the cap and cannot see the host. Published once per failed streak; WARN. Fields: fields.error, os")
}

type Publish func(signalcenter.Event)

func (p Publish) emit(origin string, code signalcenter.Code, severity signalcenter.Severity, reason string, fields map[string]string) {
	if p == nil {
		return
	}
	p(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopWave,
		Severity: severity, Code: code, Reason: reason, Fields: maps.Clone(fields)})
}
