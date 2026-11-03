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

type laneHost struct {
	lifecycle inboxmover.LedgerAppender
	forbidden func(string) bool
	signals   *signalcenter.Center
	stderr    io.Writer
}

func newLaneResume(projectRoot, evolveDir string, scope []string, h laneHost) laneResume {
	return laneResume{projectRoot: projectRoot, evolveDir: evolveDir, scope: scope, record: checkpoint.RecordLaneResume,
		claim:   hostInboxClaimer(h.lifecycle, h.signals, h.forbidden),
		release: laneClaimReleaser(projectRoot, h.stderr, h.lifecycle, h.signals), signals: h.signals, stderr: h.stderr}
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
	defer l.unlease(rp)
	return orch.RunCycleFromPhase(ctx, resumedRequest(req), rp)
}

func (l laneResume) unlease(rp *core.ResumePoint) {
	if err := core.ReleaseResumeLease(filepath.Dir(rp.StatePath)); err != nil {
		fmt.Fprintf(l.stderr, "evolve cycle run: WARN: cycle %d: cannot release the admission lease: %v\n", rp.CycleID, err)
	}
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
	if err := l.heldBy(rp.CycleID, true); err != nil {
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
		l.unlease(rp)
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
		err = l.heldBy(cycle, false)
	}
	if err == nil {
		return nil
	}
	return errors.Join(err, l.release(cycle))
}

func (l laneResume) heldBy(cycle int, pendingToo bool) error {
	for _, id := range l.scope {
		loc, err := inboxmover.Locate(l.inboxDir(), id)
		if err != nil {
			return fmt.Errorf("locate %s: %w", id, err)
		}
		if loc.Cycle != cycle && (!pendingToo || loc.Cycle != 0) {
			return fmt.Errorf("%s is not claimed by cycle %d (cycle %d holds it)", id, cycle, loc.Cycle)
		}
	}
	return nil
}

func (l laneResume) inboxDir() string {
	return filepath.Join(l.evolveDir, "inbox")
}
