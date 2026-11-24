package ship

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

type HeldAuditPort func(ctx context.Context, evolveDir, workspace string, rec heldproposal.Record) error

type HeldAuditPlanPort func(workspace string, rec heldproposal.Record) heldaudit.Plan

func auditWorkspace(opts *Options, cycle int) string {
	if opts.WorkspacePath != "" {
		return opts.WorkspacePath
	}
	return core.RunWorkspacePath(opts.ProjectRoot, cycle)
}

func dispatchHeldAudit(ctx context.Context, opts *Options, res *RunResult, shipErrValue error) {
	var se *core.ShipError
	if opts.Class != ClassCycle || !errors.As(shipErrValue, &se) || se.Code != shiperr.CodeHeldForOperator || se.Class != core.ShipClassPrecondition {
		return
	}
	cycle, err := strconv.Atoi(se.Debug["cycle"])
	if err != nil {
		return
	}
	evolveDir := filepath.Join(opts.ProjectRoot, ".evolve")
	rec, found, err := heldproposal.ReadRecord(evolveDir, cycle)
	if err != nil || !found || rec.Status != heldproposal.StatusHeld {
		res.Logs = append(res.Logs, "[ship] HELD-AUDIT: no hold record to audit for cycle "+strconv.Itoa(cycle))
		return
	}
	if shipwindow.Held(ctx) {
		res.Logs = append(res.Logs, "[ship] HELD-AUDIT: cycle "+strconv.Itoa(cycle)+": the orchestrator holds the ship window, so the audit waits for the next ship try or evolve landing audit")
		recordHeldWait(res, evolveDir, rec, heldproposal.WaitShipWindowHeld, "the ship window of the orchestrator is held")
		return
	}
	port := opts.HeldAudit
	if port == nil {
		recordHeldWait(res, evolveDir, rec, heldproposal.WaitPortNotWired, "no audit port is wired in this ship")
		return
	}
	if err := port(ctx, evolveDir, auditWorkspace(opts, cycle), rec); err != nil {
		res.Logs = append(res.Logs, "[ship] HELD-AUDIT: cycle "+strconv.Itoa(cycle)+": "+err.Error())
	}
}

func recordHeldWait(res *RunResult, evolveDir string, rec heldproposal.Record, reason, detail string) {
	dispatcher := heldaudit.Dispatcher{Deps: heldaudit.Deps{Now: time.Now}, EvolveDir: evolveDir}
	if err := dispatcher.RecordWait(rec, reason, detail); err != nil {
		res.Logs = append(res.Logs, "[ship] HELD-AUDIT: record the wait: "+err.Error())
	}
}
