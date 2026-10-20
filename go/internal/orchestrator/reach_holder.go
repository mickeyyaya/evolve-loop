package orchestrator

import (
	"fmt"
	"maps"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type ReachHolders struct {
	mu   sync.Mutex
	held map[string]admission.ShadowAdmission
}

func NewReachHolders() *ReachHolders {
	return &ReachHolders{held: map[string]admission.ShadowAdmission{}}
}

func (r *ReachHolders) Record(s admission.ShadowAdmission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.held[s.ID] = s
}

func (r *ReachHolders) gate(next Admit) Admit {
	return func(inFlight, pending []fleet.Todo) int {
		r.mu.Lock()
		clear(r.held)
		r.mu.Unlock()
		return next(inFlight, pending)
	}
}

func (r *ReachHolders) take(todo string) (admission.ShadowAdmission, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.held[todo]
	delete(r.held, todo)
	return s, ok
}

func (p Pool) carryReachHolder(td fleet.Todo, spec fleet.CycleSpec) fleet.CycleSpec {
	if p.Holders == nil {
		return spec
	}
	s, ok := p.Holders.take(td.ID)
	if !ok {
		return spec
	}
	env := make(map[string]string, len(spec.Env)+1)
	maps.Copy(env, spec.Env)
	env[ipcenv.FleetReachHolderKey] = s.HeldBy
	spec.Env = env
	p.Publish.emit("Pool.prepare", CodeAdmissionOptimistic, signalcenter.SeverityInfo,
		fmt.Sprintf("launched %s beside %s: only the optimistic rule admits it (%s)", td.ID, s.HeldBy, s.Reason),
		map[string]string{"todo": td.ID, "held_by": s.HeldBy, "reason": s.Reason})
	return spec
}

func RecordOptimisticFailure(todo, holder string, gateOf func(signalcenter.Event) (string, bool), publish Publish) signalcenter.Listener {
	var once sync.Once
	return func(e signalcenter.Event) {
		gate, failed := gateOf(e)
		if !failed {
			return
		}
		once.Do(func() {
			publish.emit("RecordOptimisticFailure", CodeOptimisticAdmissionFailed, signalcenter.SeverityWarn,
				fmt.Sprintf("lane %s, admitted beside %s only by the optimistic rule, failed %s", todo, holder, gate),
				map[string]string{"todo": todo, "held_by": holder, "gate": gate})
		})
	}
}
