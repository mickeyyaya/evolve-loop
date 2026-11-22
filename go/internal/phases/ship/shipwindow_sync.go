package ship

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainsync"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	codeShipMainFastForwarded signalcenter.Code = "SHIP_MAIN_FAST_FORWARDED"
	shipWindowSyncTimeout                       = 60 * time.Second
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, codeShipMainFastForwarded, "a ship fast-forwarded the local main of the plane to origin inside the ship window, because a ship failure named a moved main and origin held commits the local main lacked. fields.old and fields.new name the heads, fields.cycle the ship")
}

func namesMovedMain(err error) bool {
	se, ok := core.AsShipError(err)
	return ok && core.NamesMovedMain(se.Code)
}

func syncMainAfterMovedMain(ctx context.Context, opts *Options, res *RunResult, shipFailure error) {
	if !namesMovedMain(shipFailure) {
		return
	}
	if _, fromWorktree, err := landingTree(opts); err != nil || !fromWorktree || opts.DryRun {
		res.Logs = append(res.Logs, "[ship] main sync skipped: not a worktree cycle lane")
		return
	}
	if !shipwindow.Held(ctx) {
		res.Logs = append(res.Logs, "[ship] main sync skipped: ship window not held")
		return
	}
	if lost := shipwindow.HeldErr(ctx); lost != nil {
		res.Logs = append(res.Logs, "[ship] main sync skipped: ship window lease lost ("+lost.Error()+")")
		return
	}
	sctx, cancel := context.WithTimeout(ctx, shipWindowSyncTimeout)
	defer cancel()
	g := gitexec.Git{Dir: opts.ProjectRoot, Exec: opts.runner()}
	branch, err := g.Output(sctx, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) != gitexec.MainBranch {
		res.Logs = append(res.Logs, "[ship] main sync skipped: the plane is not on "+gitexec.MainBranch)
		return
	}
	reportMainSync(opts, res, mainsync.Run(sctx, g, mainsync.Policy{}))
}

func reportMainSync(opts *Options, res *RunResult, r mainsync.Result) {
	switch r.Outcome {
	case mainsync.OutcomeFetchFailed:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: main sync skipped: fetch origin failed (rc=%d %v: %s)", r.Code, r.Err, r.Stderr))
	case mainsync.OutcomeRelationFailed:
		res.Logs = append(res.Logs, "[ship] WARN: main sync skipped: "+r.Err.Error())
	case mainsync.OutcomeDiverged:
		res.Logs = append(res.Logs, "[ship] WARN: main sync: local main diverged from origin; ship does not reset or force it: "+r.Relation.String())
	case mainsync.OutcomeBlocked:
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: main sync: local tracked changes block the fast-forward (rc=%d %v: %s); the boundary sync lands them", r.Code, r.Err, r.Stderr))
	case mainsync.OutcomeFastForwarded:
		reportFastForward(opts, res, r)
	}
}

func reportFastForward(opts *Options, res *RunResult, r mainsync.Result) {
	rel := r.Relation
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] main sync: fast-forwarded main %.12s to origin/main %.12s inside the ship window", rel.Local, rel.Remote))
	opts.Signals.Emit(signalcenter.Event{
		Cycle: opts.CycleID, RunID: opts.RunID, Phase: "ship", Module: signalcenter.ModuleShip, Origin: "syncMainAfterMovedMain",
		Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityInfo, Code: codeShipMainFastForwarded,
		Reason: fmt.Sprintf("ship fast-forwarded main %.12s to origin/main %.12s", rel.Local, rel.Remote),
		Fields: map[string]string{"old": rel.Local, "new": rel.Remote, "cycle": fmt.Sprint(opts.CycleID)},
	})
	for _, a := range r.Absorbed {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] main sync: absorbed the root copy of %s into the claim of cycle-%d (%s)", a.ID, a.Cycle, a.Outcome))
	}
	if r.AbsorbErr != nil {
		res.Logs = append(res.Logs, "[ship] WARN: main sync: absorbing root copies failed: "+r.AbsorbErr.Error())
	}
}
