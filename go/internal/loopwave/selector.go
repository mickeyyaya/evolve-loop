package loopwave

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

type Selector struct {
	Only    []string
	Exclude []string
}

func (s Selector) allows(id string) bool {
	if slices.Contains(s.Exclude, id) {
		return false
	}
	return len(s.Only) == 0 || slices.Contains(s.Only, id)
}

func (s Selector) minLanes() int {
	if len(s.Only) > 0 || len(s.Exclude) > 0 {
		return 1
	}
	return 2
}

func WithSelector(s Selector) Option {
	return func(e *Engine) { e.selector = s }
}

func (e *Engine) backlog() []triagecap.FleetCandidate {
	all := triagecap.ReadInboxBacklog(e.roots.EvolveDir, e.ports.Protected)
	return slices.DeleteFunc(all, func(c triagecap.FleetCandidate) bool { return !e.selector.allows(c.ID) })
}
