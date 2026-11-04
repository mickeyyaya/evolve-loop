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
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
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
	return scopeHold{inbox: l.inboxDir(), scope: l.scope}
}

type scopeHold struct {
	inbox string
	scope []string
}

func (h scopeHold) adoptableBy(cycle int) error {
	return h.walk(cycle, func(owner int) bool { return owner == cycle || owner == 0 })
}

func (h scopeHold) heldBy(cycle int) error {
	return h.walk(cycle, func(owner int) bool { return owner == cycle })
}

func (h scopeHold) walk(cycle int, accepts func(owner int) bool) error {
	for _, id := range h.scope {
		loc, err := inboxmover.Locate(h.inbox, id)
		if err != nil {
			return fmt.Errorf("locate %s: %w", id, err)
		}
		if !accepts(loc.Cycle) {
			return fmt.Errorf("%s is not claimed by cycle %d (cycle %d holds it)", id, cycle, loc.Cycle)
		}
	}
	return nil
}

func (l laneResume) inboxDir() string {
	return filepath.Join(l.evolveDir, "inbox")
}
