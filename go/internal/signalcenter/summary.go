package signalcenter

// Summary is one listener's per-cycle view (design §8): how many signals the
// current cycle raised by severity and kind, and the last INCIDENT. It follows
// the current cycle by itself — an event for a newer cycle resets the view, an
// event for an older one is ignored, and process-level events (cycle 0) count
// toward the current cycle — so a loop process that runs many cycles needs no
// external reset. Not safe for concurrent use; the owner serializes calls.
type Summary struct {
	Cycle        int
	Total        int
	BySeverity   map[Severity]int
	ByKind       map[Kind]int
	LastIncident *Event
}

// NewSummary returns an empty view for cycle 0.
func NewSummary() *Summary { return &Summary{BySeverity: map[Severity]int{}, ByKind: map[Kind]int{}} }

// Observe folds one delivered event into the view.
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

// Snapshot returns an independent copy (maps and the last incident included);
// a zero Summary snapshots to empty, non-nil maps.
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
