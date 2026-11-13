package selfheal

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

const (
	CodeMainRedStopWouldRefuse signalcenter.Code = "GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE"
	CodeMainRedStopReleased    signalcenter.Code = "GATE_REPO_MAIN_RED_STOP_RELEASED"
	CodeMainRedStopEscalated   signalcenter.Code = "GATE_REPO_MAIN_RED_STOP_ESCALATED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainRedStopWouldRefuse, "a stop holds, the landing is not a proven fix, and gates.main_red_stop is at shadow, so ship or the train lands it and records that enforce refuses it (ADR-0165 G4); fields.keys (package.Test of each key of the stop), landing (the lane, or the batch of the train), base (the commit that the landing is on), admission (rebase when the landing is not on the main tip, not-a-fix when it turns no key green), main (the main tip for rebase, the commit of the stop for not-a-fix), fix_item")
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainRedStopReleased, "each key of the stop is green, through a fix that landed or through a rerun on the main tip, so ship or the train released the stop (ADR-0165 G4); the parked lanes resume in the next pool fill, and a fix item that is not consumed moves to processed; fields.main, fix_item, released_by (the cycle and commit of the fix, the main commit of the rerun, or the batch tip that the train moved main to), parked (the parked cycles), fix_item_error (empty when the inbox is consistent; an item that did not move makes the signal a warning)")
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainRedStopEscalated, "a stop held longer than gates.main_red_stop_bound_minutes, so the bound check escalated it one time (ADR-0165 G4); main-red-stop.json has escalated_at, and an open main-red incident under .evolve/incidents has the cause and the resume condition, so the healing catalog can propose a revert of the last landing; fields.keys (package.Test of each key of the stop), main, fix_item, parked (the parked cycles), opened_at, bound, incident (the id of the incident record)")
}
