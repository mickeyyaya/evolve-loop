//go:build acs

package cycle2585

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	systemFailureHaltExit = 4
	systemHaltCause       = "system_halt"
	operatorStopCause     = "operator_stop"
	haltDrainingCode      = signalcenter.Code("LOOP_ORCH_HALT_DRAINING")
	publishedBuffer       = 256
	orchestratorPackage   = "github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	cmdPackage            = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	callerTest            = "TestOrchestratedRun_ASystemFailureHaltLaneStopsTheBackfill"
	poolTest              = "TestPool_NoBackfillAfterSystemFailureHalt"
	causeTest             = "TestIdleCause_SystemHaltIsNotOperatorStop"
)

type gatedLanes struct {
	mu       sync.Mutex
	launched []string
	started  chan string
	gates    map[string]chan int
}

func newGatedLanes(gated ...string) *gatedLanes {
	g := &gatedLanes{started: make(chan string, len(gated)+1), gates: map[string]chan int{}}
	for _, id := range gated {
		g.gates[id] = make(chan int, 1)
	}
	return g
}

func (g *gatedLanes) launch(_ context.Context, spec fleet.CycleSpec) (int, error) {
	id := spec.Scope[0]
	g.mu.Lock()
	g.launched = append(g.launched, id)
	gate := g.gates[id]
	g.mu.Unlock()
	if gate == nil {
		return 0, nil
	}
	g.started <- id
	return <-gate, nil
}

func (g *gatedLanes) finish(id string, rc int) { g.gates[id] <- rc }

func (g *gatedLanes) launchedIDs() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := slices.Clone(g.launched)
	slices.Sort(out)
	return out
}

type poolRun struct {
	lanes   *gatedLanes
	events  []signalcenter.Event
	results []fleet.Result
}

func (r poolRun) causes(code signalcenter.Code) []string {
	var out []string
	for _, e := range r.events {
		if e.Code == code {
			out = append(out, e.Fields["cause"])
		}
	}
	return out
}

func (r poolRun) count(code signalcenter.Code) int {
	n := 0
	for _, e := range r.events {
		if e.Code == code {
			n++
		}
	}
	return n
}

type scenario struct {
	target     int
	backlog    []string
	running    []string
	firstRC    int
	halted     func() bool
	afterFirst func()
}

func runScenario(t *testing.T, sc scenario) poolRun {
	t.Helper()
	lanes := newGatedLanes(sc.running...)
	published := make(chan signalcenter.Event, publishedBuffer)
	pool := orchestrator.Pool{Target: sc.target, Launch: lanes.launch, Halted: sc.halted,
		Publish: func(e signalcenter.Event) { published <- e }}
	backlogTodos := make([]fleet.Todo, len(sc.backlog))
	for i, id := range sc.backlog {
		backlogTodos[i] = fleet.Todo{ID: id}
	}
	done := make(chan []fleet.Result, 1)
	go func() { done <- pool.Run(context.Background(), backlogTodos) }()
	for range sc.running {
		<-lanes.started
	}
	if sc.afterFirst != nil {
		sc.afterFirst()
	}
	var seen []signalcenter.Event
	lanes.finish(sc.running[0], sc.firstRC)
	for e := range published {
		seen = append(seen, e)
		if e.Code == orchestrator.CodeLaneExited && e.Fields["todo"] == sc.running[0] {
			break
		}
	}
	for _, id := range sc.running[1:] {
		lanes.finish(id, 0)
	}
	results := <-done
	close(published)
	for e := range published {
		seen = append(seen, e)
	}
	return poolRun{lanes: lanes, events: seen, results: results}
}

func haltedWave(t *testing.T) poolRun {
	t.Helper()
	return runScenario(t, scenario{target: 2, backlog: []string{"a", "b", "c", "d"}, running: []string{"a", "b"}, firstRC: systemFailureHaltExit})
}

func requireNeverLaunched(t *testing.T, r poolRun, idx ...int) {
	t.Helper()
	for _, i := range idx {
		if !errors.Is(r.results[i].Err, fleet.ErrNeverLaunched) {
			t.Errorf("results[%d] = %+v, want ErrNeverLaunched: the pool launched a todo after the system-failure halt", i, r.results[i])
		}
	}
}

func TestC2585_001_NoBackfillAfterSystemFailureHalt(t *testing.T) {
	r := haltedWave(t)
	if got := r.lanes.launchedIDs(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("launched lanes = %v, want [a b]: a lane that exited with the system-failure halt code must stop the backfill", got)
	}
	requireNeverLaunched(t, r, 2, 3)
	if r.results[0].ExitCode != systemFailureHaltExit {
		t.Errorf("results[0].ExitCode = %d, want %d: the halting lane's own result must survive", r.results[0].ExitCode, systemFailureHaltExit)
	}
}

func TestC2585_002_TheIdleCauseAfterASystemFailureHaltIsSystemHalt(t *testing.T) {
	r := haltedWave(t)
	idle := r.causes(orchestrator.CodePoolIdle)
	if len(idle) == 0 {
		t.Fatalf("%s events = 0, want the idle transitions of the drained pool", orchestrator.CodePoolIdle)
	}
	for _, cause := range idle {
		if cause == operatorStopCause {
			t.Errorf("idle cause = %q: a system-failure halt is not an operator stop", cause)
		}
		if cause != systemHaltCause {
			t.Errorf("idle cause = %q, want %q", cause, systemHaltCause)
		}
	}
	if !orchestrator.IdleCause(systemHaltCause).Valid() || !slices.Contains(orchestrator.IdleCauses, orchestrator.IdleCause(systemHaltCause)) {
		t.Errorf("%q is not in the closed set orchestrator.IdleCauses = %v", systemHaltCause, orchestrator.IdleCauses)
	}
}

func TestC2585_003_AnOperatorBrakeStillReportsOperatorStop(t *testing.T) {
	var braked atomic.Bool
	r := runScenario(t, scenario{target: 2, backlog: []string{"a", "b", "c"}, running: []string{"a", "b"}, firstRC: 0,
		halted: braked.Load, afterFirst: func() { braked.Store(true) }})
	idle := r.causes(orchestrator.CodePoolIdle)
	if len(idle) == 0 {
		t.Fatalf("%s events = 0, want the idle transitions of the braked pool", orchestrator.CodePoolIdle)
	}
	for _, cause := range idle {
		if cause != operatorStopCause {
			t.Errorf("idle cause = %q, want %q: nothing raised a system-failure halt", cause, operatorStopCause)
		}
	}
	if got := r.lanes.launchedIDs(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("launched lanes = %v, want [a b]", got)
	}
}

func TestC2585_004_AHaltWithNoOtherRunningLaneEndsTheWaveAtOnce(t *testing.T) {
	r := runScenario(t, scenario{target: 1, backlog: []string{"a", "b", "c"}, running: []string{"a"}, firstRC: systemFailureHaltExit})
	if got := r.lanes.launchedIDs(); !slices.Equal(got, []string{"a"}) {
		t.Errorf("launched lanes = %v, want [a]: the pool idled on after the last lane halted", got)
	}
	requireNeverLaunched(t, r, 1, 2)
}

func TestC2585_005_TheWaveEndsWhenTheLastRunningLaneSealsAfterAHalt(t *testing.T) {
	r := haltedWave(t)
	if len(r.results) != 4 {
		t.Fatalf("results = %d, want 4: one per backlog todo", len(r.results))
	}
	if n := r.count(orchestrator.CodeLaneExited); n != 2 {
		t.Errorf("%s events = %d, want 2: the two running lanes seal, nothing else runs", orchestrator.CodeLaneExited, n)
	}
	if n := r.count(orchestrator.CodeLaneBackfilled); n != 0 {
		t.Errorf("%s events = %d, want 0 after a system-failure halt", orchestrator.CodeLaneBackfilled, n)
	}
	if r.results[1].ExitCode != 0 || r.results[1].Err != nil {
		t.Errorf("results[1] = %+v, want the running lane to finish normally and end the wave", r.results[1])
	}
}

func TestC2585_006_AHaltRaisesTheHaltDrainingIncidentWithTheCause(t *testing.T) {
	r := haltedWave(t)
	var draining []signalcenter.Event
	for _, e := range r.events {
		if e.Code == haltDrainingCode {
			draining = append(draining, e)
		}
	}
	if len(draining) == 0 {
		t.Fatalf("%s events = 0, want an INCIDENT when the pool starts to drain after a system-failure halt", haltDrainingCode)
	}
	for _, e := range draining {
		if e.Severity != signalcenter.SeverityIncident {
			t.Errorf("%s severity = %q, want %q", haltDrainingCode, e.Severity, signalcenter.SeverityIncident)
		}
		if e.Fields["cause"] != systemHaltCause {
			t.Errorf("%s cause = %q, want %q", haltDrainingCode, e.Fields["cause"], systemHaltCause)
		}
	}
}

func TestC2585_007_APoolWithoutAHaltStillBackfillsAndRaisesNoIncident(t *testing.T) {
	r := runScenario(t, scenario{target: 2, backlog: []string{"a", "b", "c", "d"}, running: []string{"a", "b"}, firstRC: 0})
	if got := r.lanes.launchedIDs(); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("launched lanes = %v, want [a b c d]: a clean exit must keep backfilling", got)
	}
	if n := r.count(haltDrainingCode); n != 0 {
		t.Errorf("%s events = %d, want 0 without a halt", haltDrainingCode, n)
	}
	for _, cause := range r.causes(orchestrator.CodePoolIdle) {
		if cause == systemHaltCause {
			t.Errorf("idle cause = %q without a halt", cause)
		}
	}
}

func TestC2585_008_TheLoopPoolRunStopsTheBackfillAfterALaneHalts(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+callerTest+")$", cmdPackage)
	out := stdout + stderr
	if code != 0 || !strings.Contains(out, "--- PASS: "+callerTest) {
		t.Fatalf("the production caller orchestratedRun does not stop the backfill after a halt (exit %d, %v):\n%s", code, err, acsassert.OutputTail(out, 20))
	}
}

func TestC2585_009_ThePoolLevelRegressionTestsArePinnedInThePackageSuite(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^("+poolTest+"|"+causeTest+")$", orchestratorPackage)
	out := stdout + stderr
	if code != 0 {
		t.Fatalf("the pool regression tests failed (exit %d, %v):\n%s", code, err, acsassert.OutputTail(out, 20))
	}
	for _, name := range []string{poolTest, causeTest} {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s did not pass in %s:\n%s", name, orchestratorPackage, acsassert.OutputTail(out, 20))
		}
	}
}
