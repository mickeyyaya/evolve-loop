package selfheal

import (
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Evidence struct {
	ID string
	At time.Time
}

type Observation struct {
	Class     Class
	Subject   string
	Severity  signalcenter.Severity
	Evidence  []Evidence
	Protected []string
	Summary   string
}

func (o Observation) Fingerprint() string { return string(o.Class) + ":" + o.Subject }

type observations struct {
	order []string
	byKey map[string]*Observation
	seen  map[string]bool
}

func newObservations() *observations {
	return &observations{byKey: map[string]*Observation{}, seen: map[string]bool{}}
}

func (s *observations) add(o Observation, ev Evidence) {
	key := o.Fingerprint()
	if s.seen[key+" "+ev.ID] {
		return
	}
	s.seen[key+" "+ev.ID] = true
	held, ok := s.byKey[key]
	if !ok {
		o.Evidence, o.Protected = nil, slices.Clone(o.Protected)
		held = &o
		s.byKey[key] = held
		s.order = append(s.order, key)
	}
	held.Evidence = append(held.Evidence, ev)
	held.Protected = appendNew(held.Protected, o.Protected...)
	if o.Severity.Level() > held.Severity.Level() {
		held.Severity = o.Severity
	}
}

func (s *observations) list() []Observation {
	out := make([]Observation, 0, len(s.order))
	for _, key := range s.order {
		out = append(out, *s.byKey[key])
	}
	return out
}

func appendNew(dst []string, items ...string) []string {
	for _, it := range items {
		if !slices.Contains(dst, it) {
			dst = append(dst, it)
		}
	}
	return dst
}
