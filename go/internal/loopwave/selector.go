package loopwave

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

type Selector struct {
	Only    []string
	Exclude []string
	Focus   []string
}

func (s Selector) allows(id string) bool {
	if slices.Contains(s.Exclude, id) {
		return false
	}
	return len(s.Only) == 0 || slices.Contains(s.Only, id)
}

func (s Selector) minLanes() int {
	if len(s.Focus) == 0 && (len(s.Only) > 0 || len(s.Exclude) > 0) {
		return 1
	}
	return 2
}

func WithSelector(s Selector) Option {
	return func(e *Engine) { e.selector = s }
}

func (e *Engine) backlog() []triagecap.FleetCandidate {
	all := triagecap.ReadInboxBacklog(e.roots.EvolveDir, e.ports.Protected)
	return e.selector.lead(slices.DeleteFunc(all, func(c triagecap.FleetCandidate) bool { return !e.selector.allows(c.ID) }))
}

func (s Selector) lead(ranked []triagecap.FleetCandidate) []triagecap.FleetCandidate {
	if len(s.Focus) == 0 {
		return ranked
	}
	out := make([]triagecap.FleetCandidate, 0, len(ranked))
	for _, id := range s.Focus {
		if i := slices.IndexFunc(ranked, func(c triagecap.FleetCandidate) bool { return c.ID == id }); i >= 0 {
			out = append(out, ranked[i])
			ranked = slices.Delete(ranked, i, i+1)
		}
	}
	return append(out, ranked...)
}

func (s Selector) shield(started map[string]bool) map[string]bool {
	if len(s.Focus) == 0 {
		return started
	}
	out := make(map[string]bool, len(started)+len(s.Focus))
	for id, v := range started {
		out[id] = v
	}
	for _, id := range s.Focus {
		out[id] = true
	}
	return out
}

func (s Selector) focused(backlog []triagecap.FleetCandidate) []triagecap.FleetCandidate {
	var out []triagecap.FleetCandidate
	for _, c := range backlog {
		if slices.Contains(s.Focus, c.ID) {
			out = append(out, c)
		}
	}
	return out
}
