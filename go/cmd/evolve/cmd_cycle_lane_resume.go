package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type cycleStarter interface {
	RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error)
	RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error)
}

type laneResume struct {
	projectRoot, evolveDir string
	scope                  []string
	record                 func(string, bool) (checkpoint.Checkpoint, bool, error)
	claim                  deliverable.Claimer
	release                func(cycle int) error
	signals                *signalcenter.Center
	stderr                 io.Writer
	protected              func(string) bool
}

func fleetLaneScope() []string {
	if !fleetLaneFromEnv() {
		return nil
	}
	return core.ScopeIDs(os.Getenv(ipcenv.FleetScopeKey))
}

func laneClaimReleaser(projectRoot string, stderr io.Writer, lifecycle inboxmover.LedgerAppender, signals *signalcenter.Center) func(int) error {
	return func(cycle int) error {
		return releaseDeferredLaneClaims(projectRoot, cycle, laneQuotaDeferredReason, stderr, lifecycle, signals)
	}
}

func (l laneResume) run(ctx context.Context, orch cycleStarter, req core.CycleRequest) (core.CycleResult, error) {
	rp := l.adopt()
	if rp == nil {
		return orch.RunCycle(ctx, req)
	}
	defer core.ActivateResumeStatePath(rp, l.evolveDir)()
	return orch.RunCycleFromPhase(ctx, resumedRequest(req), rp)
}

func resumedRequest(req core.CycleRequest) core.CycleRequest {
	req.GoalHash = ""
	req.Context = maps.Clone(req.Context)
	delete(req.Context, "goal")
	return req
}

func (l laneResume) adopt() *core.ResumePoint {
	if len(l.scope) == 0 {
		return nil
	}
	rp, ok := core.FindLaneResume(l.projectRoot, l.evolveDir, l.scope, core.ResumeOptions{Log: l.stderr})
	if !ok {
		return nil
	}
	name := strings.Join(l.scope, ",")
	if err := l.hold().adoptableBy(rp.CycleID); err != nil {
		fmt.Fprintf(l.stderr, "evolve cycle run: lane %s: cycle %d cannot take its todo back (%v); the lane starts a fresh cycle\n", name, rp.CycleID, err)
		return nil
	}
	cp, admitted, err := l.record(rp.StatePath, true)
	if err != nil || !admitted {
		fmt.Fprintf(l.stderr, "evolve cycle run: lane %s: the resume of cycle %d was refused (admitted %v, err %v); the lane starts a fresh cycle\n", name, rp.CycleID, admitted, err)
		return nil
	}
	if err := l.reclaim(rp.CycleID); err != nil {
		fmt.Fprintf(l.stderr, "evolve cycle run: lane %s: cycle %d cannot take its todo back (%v); the lane starts a fresh cycle\n", name, rp.CycleID, err)
		return nil
	}
	l.binding().settle(rp.CycleID)
	rp.AutoAttempts = cp.AutoResumeAttempts
	core.EmitLaneResumed(l.signals, rp, l.scope)
	fmt.Fprintf(l.stderr, "evolve cycle run: lane %s resumes cycle %d at phase %s (auto-resume %d of %d)\n", name, rp.CycleID, rp.Phase, rp.AutoAttempts, rp.AutoMaxAttempts)
	return rp
}

func (l laneResume) reclaim(cycle int) error {
	err := l.claim(l.inboxDir(), cycle, l.scope)
	if err == nil {
		err = l.hold().heldBy(cycle)
	}
	if err == nil {
		return nil
	}
	return errors.Join(err, l.release(cycle))
}

func (l laneResume) hold() scopeHold {
	return scopeHold{inbox: l.inboxDir(), scope: l.scope, protected: l.protected}
}

type scopeHold struct {
	inbox     string
	scope     []string
	protected func(string) bool
}

func (h scopeHold) adoptableBy(cycle int) error {
	return h.walk(cycle, func(owner int) bool { return owner == cycle || owner == 0 }, h.notConsoleRouted)
}

func (h scopeHold) heldBy(cycle int) error {
	return h.walk(cycle, func(owner int) bool { return owner == cycle }, func(string, string) error { return nil })
}

func (h scopeHold) walk(cycle int, accepts func(owner int) bool, check func(id, path string) error) error {
	for _, id := range h.scope {
		loc, err := inboxmover.Locate(h.inbox, id)
		if err != nil {
			return fmt.Errorf("locate %s: %w", id, err)
		}
		if !accepts(loc.Cycle) {
			return fmt.Errorf("%s is not claimed by cycle %d (cycle %d holds it)", id, cycle, loc.Cycle)
		}
		if err := check(id, loc.Path); err != nil {
			return err
		}
	}
	return nil
}

func (h scopeHold) notConsoleRouted(id, path string) error {
	item, _, err := inboxbatch.LoadFile(path)
	if err != nil {
		return nil
	}
	if routed, reason := inboxbatch.ConsoleRouted(item, h.protected); routed {
		return fmt.Errorf("%s is console-routed (%s): the operator works it, no lane resumes it", id, reason)
	}
	return nil
}

func (l laneResume) inboxDir() string {
	return filepath.Join(l.evolveDir, "inbox")
}

type scopeBinding struct {
	projectRoot string
	stderr      io.Writer
	signals     *signalcenter.Center
	release     func(inboxmover.ReleaseRequest) (bool, error)
}

type pendingBinding struct {
	ws       string
	cycle    int
	id       string
	c        continuation.Continuation
	standing string
}

func (l laneResume) binding() scopeBinding {
	opts := inboxmover.Options{ProjectRoot: l.projectRoot, InboxDir: l.inboxDir(), Stderr: l.stderr, Signals: l.signals}
	return scopeBinding{projectRoot: l.projectRoot, stderr: l.stderr, signals: l.signals, release: func(req inboxmover.ReleaseRequest) (bool, error) {
		req.By = "runtime (lane resume)"
		_, released, err := inboxmover.ReleaseContinuationBindingFor(opts, req)
		return released, err
	}}
}

func (b scopeBinding) settle(cycle int) {
	ws := paths.RunWorkspace(b.projectRoot, cycle)
	if _, has, err := continuation.ReadManifest(ws); err != nil || has {
		return
	}
	ids := core.LaneScopeIDs(ws)
	for range ids {
		id, c, ok := defectledger.FirstBinding(b.projectRoot, ids)
		if !ok || c.Cycle == cycle {
			return
		}
		p := pendingBinding{ws: ws, cycle: cycle, id: id, c: c}
		p.standing = b.standing(p)
		if p.standing != standingAbsent && p.standing != standingSettled || !b.releaseBinding(p) {
			b.recordBinding(p)
			return
		}
	}
}

const (
	standingAbsent     = "absent"
	standingSettled    = "settled"
	standingOwed       = "owed"
	standingUnreadable = "unreadable"
	standingNewer      = "newer"
)

func (b scopeBinding) standing(p pendingBinding) string {
	if p.c.Cycle > p.cycle {
		return standingNewer
	}
	st, err := defectledger.AncestorStanding(b.projectRoot, p.c.Cycle)
	switch {
	case err != nil:
		fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d cannot read the ledger of cycle-%d (%v); it records the binding of %s\n", p.cycle, p.c.Cycle, err, p.id)
		return standingUnreadable
	case st == defectledger.StandingOwed:
		return standingOwed
	case st == defectledger.StandingSettled:
		return standingSettled
	}
	return standingAbsent
}

func (b scopeBinding) releaseBinding(p pendingBinding) bool {
	reason := fmt.Sprintf("lane-resume-cycle-%d: ancestor cycle-%d owes no defect (ledger %s) and the resumed cycle supersedes its snapshot", p.cycle, p.c.Cycle, p.standing)
	released, err := b.release(inboxmover.ReleaseRequest{ScopeID: p.id, Cycle: p.c.Cycle, Reason: reason})
	if err != nil || !released {
		fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d cannot release the binding of %s to cycle-%d (released %v, err %v); it records the binding\n", p.cycle, p.id, p.c.Cycle, released, err)
		return false
	}
	fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d RELEASED the binding of %s to cycle-%d: the ancestor owes no defect (ledger %s)\n", p.cycle, p.id, p.c.Cycle, p.standing)
	b.settled(p, "released")
	return true
}

func (b scopeBinding) recordBinding(p pendingBinding) {
	if err := continuation.WriteManifest(p.ws, p.c); err != nil {
		fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d cannot record the binding of %s to cycle-%d in its manifest (%v); the defect-ledger gate blocks the audit\n", p.cycle, p.id, p.c.Cycle, err)
		return
	}
	fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d RECORDED the binding of %s to cycle-%d in its manifest before its resume\n", p.cycle, p.id, p.c.Cycle)
	b.settled(p, "recorded")
}

func (b scopeBinding) settled(p pendingBinding, action string) {
	core.EmitLaneBindingSettled(b.signals, core.BindingSettlement{Cycle: p.cycle, Ancestor: p.c.Cycle, ScopeID: p.id, Action: action, Standing: p.standing})
}
