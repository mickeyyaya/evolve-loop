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
		return nil
	}
	l.binding().settle(rp.CycleID, l.scope)
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

type scopeBinding struct {
	projectRoot string
	stderr      io.Writer
	release     func(scopeID, reason string) (bool, error)
}

func (l laneResume) binding() scopeBinding {
	return scopeBinding{projectRoot: l.projectRoot, stderr: l.stderr, release: func(id, reason string) (bool, error) {
		_, released, err := inboxmover.ReleaseContinuationBinding(inboxmover.Options{ProjectRoot: l.projectRoot, InboxDir: l.inboxDir(), Stderr: l.stderr, Signals: l.signals}, id, reason, "runtime (lane resume)")
		return released, err
	}}
}

func (b scopeBinding) settle(cycle int, scope []string) {
	ws := paths.RunWorkspace(b.projectRoot, cycle)
	if _, has, err := continuation.ReadManifest(ws); err != nil || has {
		return
	}
	for _, id := range scope {
		c, ok, err := continuation.ReadRegistryEntry(b.projectRoot, id)
		if err != nil || !ok {
			continue
		}
		if c.Cycle == cycle {
			return
		}
		if !b.releasable(cycle, id, c) {
			b.adopt(ws, cycle, id, c)
			return
		}
	}
}

func (b scopeBinding) releasable(cycle int, id string, c continuation.Continuation) bool {
	if c.Cycle > cycle {
		return false
	}
	if owes, err := defectledger.AncestorOwes(b.projectRoot, c.Cycle); err != nil || owes {
		return false
	}
	released, err := b.release(id, fmt.Sprintf("lane-resume-cycle-%d: ancestor cycle-%d left no defect ledger and the resumed cycle supersedes its snapshot", cycle, c.Cycle))
	if err != nil || !released {
		fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d cannot release the binding of %s to cycle-%d (released %v, err %v); it adopts the binding\n", cycle, id, c.Cycle, released, err)
		return false
	}
	fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d RELEASED the binding of %s to cycle-%d: the ancestor left no defect ledger\n", cycle, id, c.Cycle)
	return true
}

func (b scopeBinding) adopt(ws string, cycle int, id string, c continuation.Continuation) {
	if err := continuation.WriteManifest(ws, c); err != nil {
		fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d cannot adopt the binding of %s to cycle-%d (%v); the defect-ledger gate blocks the audit\n", cycle, id, c.Cycle, err)
		return
	}
	fmt.Fprintf(b.stderr, "evolve cycle run: cycle %d ADOPTED the binding of %s to cycle-%d before its resume\n", cycle, id, c.Cycle)
}
