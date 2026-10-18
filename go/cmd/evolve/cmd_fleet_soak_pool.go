package main

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const soakLanesPerSlot = 4

type soakPoolReport struct {
	Lanes int
	OK    int
	Peak  int
	Wall  time.Duration
	Busy  time.Duration
	width int
}

func (r soakPoolReport) Throughput() float64 {
	if r.Wall <= 0 {
		return 0
	}
	return float64(r.Lanes) / r.Wall.Minutes()
}

func (r soakPoolReport) Utilization() float64 {
	if r.Wall <= 0 || r.width <= 0 {
		return 0
	}
	return r.Busy.Seconds() / (float64(r.width) * r.Wall.Seconds())
}

func (r soakPoolReport) Passed(width int) bool {
	return r.Lanes > 0 && r.OK == r.Lanes && r.Peak == width
}

type soakMeter struct {
	mu   sync.Mutex
	now  func() time.Time
	last time.Time
	live int
	rep  soakPoolReport
}

func (m *soakMeter) observe(e signalcenter.Event) {
	live, _ := strconv.Atoi(e.Fields["live"])
	m.mu.Lock()
	defer m.mu.Unlock()
	at := m.now()
	m.rep.Busy += time.Duration(m.live) * at.Sub(m.last)
	m.last, m.live, m.rep.Peak = at, live, max(m.rep.Peak, live)
}

func soakHeartbeat(orchestrator.Lane) (time.Time, bool) { return time.Time{}, false }

func soakPool(ctx context.Context, count int, goalHash string, launch fleet.LaunchFn, now func() time.Time) soakPoolReport {
	backlog := make([]fleet.Todo, soakLanesPerSlot*count)
	for i := range backlog {
		backlog[i] = fleet.Todo{ID: "soak-" + strconv.Itoa(i+1)}
	}
	m := &soakMeter{now: now}
	start := now()
	m.last = start
	pool := orchestrator.Pool{
		Target: count, Concurrency: count, MaxLanes: len(backlog),
		Freshen: func(spec fleet.CycleSpec) (fleet.CycleSpec, bool) {
			spec.Env = maps.Clone(spec.Env)
			delete(spec.Env, ipcenv.FleetScopeKey)
			spec.GoalHash = goalHash
			return spec, true
		},
		Launch: launch,
		Leases: orchestrator.NewLeases(orchestrator.LeaseConfig{Stage: orchestrator.StageEnforce, Deadline: time.Hour, Stall: time.Hour},
			orchestrator.SystemClock{}, soakHeartbeat, nil),
		Publish: m.observe,
	}
	results := pool.Run(ctx, backlog)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rep.Wall, m.rep.width, m.rep.Lanes = now().Sub(start), count, len(results)
	m.rep.OK = laneTally(results).OK
	return m.rep
}

func soakPoolRow(rep soakPoolReport, count int) (status, evidence string) {
	status = "FAIL"
	if rep.Passed(count) {
		status = "PASS"
	}
	return status, fmt.Sprintf("%d lanes, peak in-flight %d of %d, %d ok, %.1f lanes/min over %s, utilization %.2f",
		rep.Lanes, rep.Peak, count, rep.OK, rep.Throughput(), rep.Wall.Round(time.Millisecond), rep.Utilization())
}
