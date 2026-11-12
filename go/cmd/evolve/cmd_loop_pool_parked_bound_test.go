package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type boundPool struct {
	parkedPool
	center *signalcenter.Center
	events []signalcenter.Event
	stderr bytes.Buffer
}

func newBoundPool(t *testing.T, openedBefore time.Duration) *boundPool {
	t.Helper()
	b := &boundPool{parkedPool: newParkedPool(t), center: signalcenter.New()}
	b.center.Subscribe(func(e signalcenter.Event) {
		if e.Code == core.CodeMainRedStopEscalated {
			b.events = append(b.events, e)
		}
	})
	b.park(2242, core.MainRedReason, "parked-a")
	b.park(2249, core.MainRedReason, "parked-b")
	tip := b.repo.Git("rev-parse", "HEAD")
	if _, err := mainstop.Update(b.evolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		opened := s.Opened(baseattr.Verdict{MainRed: []baseattr.Key{parkedKey}, Main: tip}, mainstop.FixItemID(tip), time.Now().Add(-openedBefore))
		return opened.WithParked(mainstop.Parked{Cycle: 2242}).WithParked(mainstop.Parked{Cycle: 2249}), nil
	}); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *boundPool) fill(t *testing.T) {
	t.Helper()
	plan := poolPlanner{cfg: loopConfig{ProjectRoot: b.repo.Dir, EvolveDir: b.evolveDir}, storage: &erroringReadStorage{}, stderr: &b.stderr, signals: b.center}.plan(2)
	if _, err := plan(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
}

func (b *boundPool) stop(t *testing.T) mainstop.Stop {
	t.Helper()
	stop, _, err := mainstop.Read(b.evolveDir)
	if err != nil {
		t.Fatal(err)
	}
	return stop
}

func (b *boundPool) incidents(t *testing.T) []selfheal.Incident {
	t.Helper()
	incidents, err := selfheal.LoadIncidents(selfheal.Dir(b.evolveDir))
	if err != nil {
		t.Fatal(err)
	}
	return incidents
}

func TestStop_PastTheBoundEscalatesOnceWithCauseAndResumeCondition(t *testing.T) {
	t.Parallel()
	b := newBoundPool(t, 3*time.Hour)
	held := b.stop(t)
	before := time.Now()

	b.fill(t)
	b.fill(t)

	incidents := b.incidents(t)
	if len(incidents) != 1 || incidents[0].Class != selfheal.ClassMainRed || incidents[0].State != selfheal.StateOpen {
		t.Fatalf("incidents = %+v, want one open main-red incident after two pool fills", incidents)
	}
	if !strings.Contains(incidents[0].Summary, held.Cause()) || !strings.Contains(incidents[0].Summary, held.ResumeCondition()) || !strings.Contains(incidents[0].Summary, "2242, 2249") {
		t.Errorf("summary = %q, want the cause %q and the resume condition %q", incidents[0].Summary, held.Cause(), held.ResumeCondition())
	}
	stop := b.stop(t)
	if stop.Escalated == nil || stop.Escalated.Before(before) || stop.Escalated.After(time.Now()) || !stop.Holds() {
		t.Errorf("stop = %+v, want a stop that holds with escalated_at at the time of the first fill", stop)
	}
	if len(b.events) != 1 || b.events[0].Kind != signalcenter.KindLoopEscalation || b.events[0].Severity != signalcenter.SeverityIncident ||
		b.events[0].Fields["incident"] != incidents[0].ID || b.events[0].Fields["bound"] != "2h0m0s" || b.events[0].Fields["parked"] != "2242, 2249" {
		t.Errorf("signals = %+v, want one GATE_REPO_MAIN_RED_STOP_ESCALATED with the incident %s and the default bound", b.events, incidents[0].ID)
	}
	if strings.Contains(b.stderr.String(), "[pool] WARN main-red stop") {
		t.Errorf("stderr = %q, want no warning", b.stderr.String())
	}
}

func TestStop_ThePoolTakesTheBoundFromThePolicy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		policy       string
		openedBefore time.Duration
		want         int
	}{
		{name: "inside the default bound", openedBefore: time.Hour, want: 0},
		{name: "inside a longer bound of the policy", policy: `{"gates":{"main_red_stop_bound_minutes":600}}`, openedBefore: 3 * time.Hour, want: 0},
		{name: "past a shorter bound of the policy", policy: `{"gates":{"main_red_stop_bound_minutes":30}}`, openedBefore: time.Hour, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newBoundPool(t, tc.openedBefore)
			if tc.policy != "" {
				writeE2File(t, filepath.Join(b.evolveDir, "policy.json"), tc.policy)
			}

			b.fill(t)

			if len(b.events) != tc.want || len(b.incidents(t)) != tc.want || (b.stop(t).Escalated != nil) != (tc.want == 1) {
				t.Errorf("%d signals, %d incidents, escalated_at %v; want %d escalation", len(b.events), len(b.incidents(t)), b.stop(t).Escalated, tc.want)
			}
		})
	}
}

func TestStop_AnEscalationThatFailsIsAPoolWarningAndTheLanesStayParked(t *testing.T) {
	t.Parallel()
	b := newBoundPool(t, 3*time.Hour)
	writeE2File(t, selfheal.Dir(b.evolveDir), "a file, not a directory")

	todos := poolPlanFor(t, b.repo.Dir, b.evolveDir, &b.stderr)

	if !strings.Contains(b.stderr.String(), "[pool] WARN main-red stop: the escalation of the stop past its bound") {
		t.Errorf("stderr = %q, want the warning of the failed escalation", b.stderr.String())
	}
	if ids := todoIDs(todos); strings.Contains(strings.Join(ids, ","), "parked-") || b.stop(t).Escalated != nil {
		t.Errorf("todos = %v, escalated_at %v; want the lanes parked and the stop left for the next fill", ids, b.stop(t).Escalated)
	}
}

func TestStop_TheLoopGivesItsSignalCenterToThePoolSoTheEscalationIsEmitted(t *testing.T) {
	t.Parallel()
	b := newBoundPool(t, 3*time.Hour)
	console := &lockedBuffer{}
	loop := &loopBatchCoordinator{
		ctx:       context.Background(),
		cfg:       loopConfig{ProjectRoot: b.repo.Dir, EvolveDir: b.evolveDir, GoalHash: "bound-wiring"},
		result:    &loopResult{},
		stdout:    console,
		stderr:    console,
		loadGraph: noModuleGraph,
	}
	loop.deps.Signals = b.center
	loop.deps.Storage = &erroringReadStorage{}

	loop.dispatchPool(1, poolPreflightFleet, loop.cycleLaunch("/nonexistent-binary", 0))
	b.center.Flush()

	if len(b.events) != 1 || len(b.incidents(t)) != 1 || b.stop(t).Escalated == nil {
		t.Errorf("%d escalation signals, %d incidents, escalated_at %v; want one escalation from the pool of the loop (console %q)",
			len(b.events), len(b.incidents(t)), b.stop(t).Escalated, console.String())
	}
}

func TestStop_PastTheBoundEscalatesWhenMainDoesNotResolve(t *testing.T) {
	t.Parallel()
	b := newBoundPool(t, 3*time.Hour)
	b.repo.Git("branch", "-m", "main", "trunk")

	b.fill(t)

	if len(b.events) != 1 || len(b.incidents(t)) != 1 || b.stop(t).Escalated == nil {
		t.Errorf("%d signals, %d incidents, escalated_at %v; want one escalation", len(b.events), len(b.incidents(t)), b.stop(t).Escalated)
	}
	if !strings.Contains(b.stderr.String(), "so the parked lanes stay parked") {
		t.Errorf("stderr = %q, want the warning that main does not resolve", b.stderr.String())
	}
}
