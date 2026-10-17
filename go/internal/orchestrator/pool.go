package orchestrator

import (
	"context"
	"strconv"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const ParkReason = "deadline twice"

const releaseReason = "orchestrator: the lane passed its lease deadline and was stopped"

type Claims interface {
	Release(todo, reason string) error
	Park(todo, reason string) error
}

type Admit func(inFlight, pending []fleet.Todo) int

type Pool struct {
	Target      int
	Concurrency int
	MaxLanes    int
	Admit       Admit
	Freshen     func(fleet.CycleSpec) (fleet.CycleSpec, bool)
	Launch      fleet.LaunchFn
	Leases      *Leases
	Claims      Claims
	Reap        func(Lane)
	Publish     Publish
	Halted      func() bool
}

var transitionCodes = map[fleet.PoolTransitionKind]signalcenter.Code{
	fleet.PoolLaunched:   CodeLaneLaunched,
	fleet.PoolExited:     CodeLaneExited,
	fleet.PoolBackfilled: CodeLaneBackfilled,
	fleet.PoolIdle:       CodePoolIdle,
}

func (p Pool) Run(ctx context.Context, backlog []fleet.Todo) []fleet.Result {
	var mu sync.Mutex
	retry := map[string]bool{}
	launch := p.Launch
	if p.Leases != nil {
		launch = p.Leases.Watch(p.Launch, func(lane Lane, ret Return) {
			again := p.recover(lane, ret)
			mu.Lock()
			retry[lane.Key] = again
			mu.Unlock()
		})
	}
	cfg := fleet.PoolConfig{Target: p.Target, Concurrency: p.Concurrency, MaxLaunches: p.MaxLanes, Halted: p.Halted,
		Next: p.Admit, Prepare: p.prepare,
		Exited: func(td fleet.Todo, _ fleet.Result) bool {
			mu.Lock()
			defer mu.Unlock()
			again := retry[td.ID]
			delete(retry, td.ID)
			return again
		}}
	return fleet.RunPool(ctx, cfg, backlog, launch, p.transition)
}

func (p Pool) prepare(_ fleet.Todo, spec fleet.CycleSpec) (fleet.CycleSpec, bool) {
	if p.Freshen == nil {
		return spec, true
	}
	return p.Freshen(spec)
}

func (p Pool) transition(t fleet.PoolTransition) {
	fields := map[string]string{"live": strconv.Itoa(t.Live), "target": strconv.Itoa(t.Target)}
	reason := "pool " + string(t.Kind)
	if t.Todo != "" {
		fields["todo"] = t.Todo
		reason += " " + t.Todo
	}
	p.Publish.emit("Pool.Run", transitionCodes[t.Kind], signalcenter.SeverityInfo, reason, fields)
}

func (p Pool) recover(lane Lane, ret Return) bool {
	if !ret.Stopped {
		return false
	}
	if p.Reap != nil {
		p.Reap(lane)
	}
	if p.Claims == nil {
		return false
	}
	todo := lane.Key
	p.check(todo, "release", p.Claims.Release(todo, releaseReason))
	if ret.Stops < 2 {
		return true
	}
	p.check(todo, "park", p.Claims.Park(todo, ParkReason))
	return false
}

func (p Pool) check(todo, step string, err error) {
	if err == nil {
		return
	}
	p.Publish.emit("Pool.recover", CodeLaneRecoveryFailed, signalcenter.SeverityWarn,
		"orchestrator: "+step+" of "+todo+" after its deadline stop failed: "+err.Error(),
		map[string]string{"todo": todo, "step": step, "error": err.Error()})
}

func AdmitByCell(a admission.Admitter) Admit {
	return func(inFlight, pending []fleet.Todo) int {
		holdings := make([]admission.Holding, len(inFlight))
		for i, td := range inFlight {
			holdings[i] = admission.Holding{Lane: td.ID, Todo: td}
		}
		admitted, _ := a.Admit(holdings, pending, 1)
		if len(admitted) == 0 {
			return -1
		}
		return admitted[0].Rank
	}
}
