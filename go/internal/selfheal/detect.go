package selfheal

import (
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	EscalateAt = 3
	MaxSources = 20
)

const (
	TransitionOpened   = "opened"
	TransitionReopened = "reopened"
	TransitionClosed   = "closed"
)

type Transition struct {
	Kind     string
	Incident Incident
}

type Verifier interface {
	ReproGreen(Incident) bool
	WaveEndedAfter(time.Time) bool
	RetrospectiveExists(id string) bool
}

func Detect(existing []Incident, obs []Observation, v Verifier, now time.Time) ([]Incident, []Transition) {
	records := map[string]Incident{}
	for _, inc := range existing {
		records[inc.Fingerprint] = inc
	}
	present := map[string]bool{}
	var transitions []Transition
	for _, o := range obs {
		fp := o.Fingerprint()
		prev, found := records[fp]
		next, kind := observe(prev, found, o, now)
		records[fp], present[fp] = next, true
		if kind != "" {
			transitions = append(transitions, Transition{Kind: kind, Incident: next})
		}
	}
	for fp, inc := range records {
		if inc.State != StateOpen || present[fp] {
			continue
		}
		closed, missing := inc.Close(conditionsOf(inc, obs, v), now)
		if len(missing) == 0 {
			records[fp] = closed
			transitions = append(transitions, Transition{Kind: TransitionClosed, Incident: closed})
		}
	}
	out := make([]Incident, 0, len(records))
	for _, inc := range records {
		out = append(out, inc)
	}
	slices.SortFunc(out, func(a, b Incident) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(transitions, func(a, b Transition) int { return strings.Compare(a.Incident.ID, b.Incident.ID) })
	return out, transitions
}

func observe(prev Incident, found bool, o Observation, now time.Time) (Incident, string) {
	kind := ""
	switch {
	case !found:
		prev = Incident{Schema: RecordSchema, ID: incidentID(o.Fingerprint(), now), Fingerprint: o.Fingerprint(), State: StateOpen, OpenedAt: now, Sources: []string{}}
		kind = TransitionOpened
	case prev.State == StateClosed:
		prev.State, prev.ClosedAt, prev.Reopened = StateOpen, nil, prev.Reopened+1
		kind = TransitionReopened
	}
	if newer := newEvidence(prev, found, o.Evidence); len(newer) > 0 {
		prev = absorb(prev, o, newer)
	}
	return prev, kind
}

func newEvidence(prev Incident, found bool, evidence []Evidence) []Evidence {
	var out []Evidence
	for _, ev := range evidence {
		if !found || ev.At.After(prev.LastSeenAt) || (ev.At.Equal(prev.LastSeenAt) && !slices.Contains(prev.Sources, ev.ID)) {
			out = append(out, ev)
		}
	}
	slices.SortStableFunc(out, func(a, b Evidence) int { return a.At.Compare(b.At) })
	return out
}

func absorb(inc Incident, o Observation, newer []Evidence) Incident {
	inc.Class, inc.Subject, inc.Summary = o.Class, o.Subject, o.Summary
	inc.Recurrence += len(newer)
	sources := slices.Clone(inc.Sources)
	for _, ev := range newer {
		sources = append(sources, ev.ID)
	}
	inc.Sources = sources[max(0, len(sources)-MaxSources):]
	if last := newer[len(newer)-1].At; last.After(inc.LastSeenAt) {
		inc.LastSeenAt = last
	}
	if inc.Tier != TierKernel {
		inc.Tier = tierOf(o.Class, o.Protected)
	}
	inc.Severity = rate(inc, o.Severity)
	return inc
}

func rate(inc Incident, observed signalcenter.Severity) signalcenter.Severity {
	if inc.Recurrence >= EscalateAt || inc.Tier == TierKernel {
		return signalcenter.SeverityIncident
	}
	if observed.Level() > inc.Severity.Level() {
		return observed
	}
	return inc.Severity
}

func conditionsOf(inc Incident, obs []Observation, v Verifier) Conditions {
	return Conditions{
		ReproGreen:    v.ReproGreen(inc),
		SignalClear:   v.WaveEndedAfter(inc.LastSeenAt),
		QualityHeld:   !qualityRegressedAfter(obs, inc),
		Retrospective: v.RetrospectiveExists(inc.ID),
	}
}

func qualityRegressedAfter(obs []Observation, inc Incident) bool {
	for _, o := range obs {
		if o.Class != ClassQuality {
			continue
		}
		if slices.ContainsFunc(o.Evidence, func(ev Evidence) bool { return ev.At.After(inc.OpenedAt) }) {
			return true
		}
	}
	return false
}
