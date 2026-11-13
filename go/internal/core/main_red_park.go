package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	MainRedReason = "main-red"

	CodeMainRedParked     signalcenter.Code = "GATE_REPO_MAIN_RED_PARKED"
	CodeMainRedStopOpened signalcenter.Code = "GATE_REPO_MAIN_RED_STOP_OPENED"

	wallPauseCause   = "the dispatch chain ended at a wall"
	mainRedParkCause = "ship parked the lane on a red main"

	unjudgedMainCause  = "its red tests are red on the lane base, and ship did not judge main"
	unjudgedMainResume = "the lane resumes in the next pool fill, and ship judges main again"
)

var MainRedBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

var errNoMainRedCheckpointer = errors.New("no main-red checkpointer is wired")

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainRedParked, "ship found the red tests of the lane red on its base and on main, so the orchestrator parked the cycle and did not blame the lane (ADR-0165 G4); the cycle has a main-red checkpoint and a row in main-red-stop.json, and it resumes when main is green; fields.keys (package.Test of each key of the stop), main, fix_item, parked (the parked cycles), checkpoint_error and stop_error (each empty when its write passed; a write that failed makes the signal an incident, because the cycle cannot resume from the park)")
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainRedStopOpened, "a build floor or ship found tests that are red on main, and no stop held, so the stop opened (ADR-0165 G4); main-red-stop.json has the keys and the fix item is in the inbox, where it leads the rank while the stop holds; fields.keys (package.Test of each key that is red on main), main, fix_item, fix_item_error (empty when the item is in the inbox; an item that was not filed makes the signal an incident)")
}

func isParked(err error) bool {
	return errors.Is(err, ErrAllFamiliesExhausted) || errors.Is(err, ErrMainRedParked)
}

func parkCause(err error) string {
	if errors.Is(err, ErrMainRedParked) {
		return mainRedParkCause
	}
	return wallPauseCause
}

func writeMainRedCheckpoint(cs CycleState, projectRoot string, now time.Time) error {
	if MainRedBoundaryCheckpointer == nil {
		return errNoMainRedCheckpointer
	}
	return MainRedBoundaryCheckpointer(cs, projectRoot, now)
}

func parkInStop(parked mainstop.Parked) func(mainstop.Stop, bool) (mainstop.Stop, error) {
	return func(s mainstop.Stop, _ bool) (mainstop.Stop, error) { return s.WithParked(parked), nil }
}

func (o *Orchestrator) parkOnMainRed(projectRoot string, cs CycleState, v baseattr.Verdict) {
	now, cycle := o.now(), cs.CycleID
	checkpointErr := writeMainRedCheckpoint(cs, projectRoot, now)
	parked := mainstop.Parked{Cycle: cycle, Scope: LaneScopeIDs(cs.WorkspacePath), Workspace: cs.WorkspacePath}
	opening := stopOpening{projectRoot: projectRoot, verdict: v, cycle: cycle, runID: cs.RunID, phase: string(PhaseShip), origin: "Orchestrator.parkOnMainRed"}
	openErr := opening.through(mainRedStopOpener{now: o.now}.open, o.signals)
	stop, parkErr := mainstop.Update(filepath.Join(projectRoot, ".evolve"), parkInStop(parked))
	stopErr := errors.Join(openErr, parkErr)
	e := signalcenter.Event{
		Cycle: cycle, RunID: cs.RunID, Phase: string(PhaseShip),
		Module: signalcenter.ModuleGateRepo, Origin: "Orchestrator.parkOnMainRed", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeMainRedParked,
		Reason: parkReason(cycle, stop),
		Fields: map[string]string{
			"keys": baseattr.Names(stop.Keys), "main": stop.MainSHA, "fix_item": stop.FixItem, "parked": stop.ParkedCycles(),
			"checkpoint_error": errText(checkpointErr), "stop_error": errText(stopErr),
		},
	}
	if failed := errors.Join(checkpointErr, stopErr); failed != nil {
		cause := strings.ReplaceAll(failed.Error(), "\n", "; ")
		e.Severity = signalcenter.SeverityIncident
		e.Reason = fmt.Sprintf("cycle %d met a red main, but the park is not whole (%s), so the cycle cannot resume from it; next: read the error, repair the write, then start the item again", cycle, cause)
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN main-red park of cycle %d is not whole: %s\n", cycle, cause)
	}
	o.signals.Emit(e)
}

func parkReason(cycle int, stop mainstop.Stop) string {
	cause, next := stop.Cause(), stop.ResumeCondition()
	if !stop.Holds() {
		cause, next = unjudgedMainCause, unjudgedMainResume
	}
	return fmt.Sprintf("cycle %d is parked and its lane is not blamed: %s; next: %s", cycle, cause, next)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
