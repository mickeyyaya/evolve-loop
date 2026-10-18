package orchestrator

import (
	"slices"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

type Intake struct {
	mu      sync.Mutex
	freed   map[string]bool
	stopped bool
	limit   int
	kick    chan struct{}
}

func NewIntake(limit int) *Intake {
	return &Intake{freed: map[string]bool{}, limit: limit, kick: make(chan struct{}, 1)}
}

func (in *Intake) Free(lane string) {
	in.mu.Lock()
	in.freed[lane] = true
	in.mu.Unlock()
	in.nudge()
}

func (in *Intake) Hold(lane string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	delete(in.freed, lane)
}

func (in *Intake) Stop() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.stopped = true
}

func (in *Intake) Stopped() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.stopped
}

func (in *Intake) Resize(limit int) {
	in.mu.Lock()
	in.limit = limit
	in.mu.Unlock()
	in.nudge()
}

func (in *Intake) Wake() <-chan struct{} { return in.kick }

func (in *Intake) Gate(next Admit) Admit {
	return func(inFlight, pending []fleet.Todo) int {
		in.mu.Lock()
		room := len(inFlight) < in.limit
		held := slices.DeleteFunc(slices.Clone(inFlight), func(td fleet.Todo) bool { return in.freed[td.ID] })
		in.mu.Unlock()
		if !room {
			return -1
		}
		return next(held, pending)
	}
}

func (in *Intake) nudge() {
	select {
	case in.kick <- struct{}{}:
	default:
	}
}
