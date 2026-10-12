package signalcenter

type Summary struct {
	Cycle        int
	Total        int
	BySeverity   map[Severity]int
	ByKind       map[Kind]int
	LastIncident *Event
}

func NewSummary() *Summary { return &Summary{BySeverity: map[Severity]int{}, ByKind: map[Kind]int{}} }

func (s *Summary) Observe(e Event) {
	if s.BySeverity == nil {
		*s = *NewSummary()
	}
	switch {
	case e.Cycle > s.Cycle:
		*s = *NewSummary()
		s.Cycle = e.Cycle
	case e.Cycle != 0 && e.Cycle < s.Cycle:
		return
	}
	s.Total++
	s.BySeverity[e.Severity]++
	s.ByKind[e.Kind]++
	if e.Severity == SeverityIncident {
		last := e
		s.LastIncident = &last
	}
}

func (s *Summary) Snapshot() Summary {
	out := Summary{Cycle: s.Cycle, Total: s.Total, BySeverity: map[Severity]int{}, ByKind: map[Kind]int{}}
	for k, v := range s.BySeverity {
		out.BySeverity[k] = v
	}
	for k, v := range s.ByKind {
		out.ByKind[k] = v
	}
	if s.LastIncident != nil {
		last := *s.LastIncident
		out.LastIncident = &last
	}
	return out
}
