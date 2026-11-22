package wavehealth

import (
	"fmt"
	"sort"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

type kindTally struct {
	kind   string
	events int
	cycles map[int]bool
}

func abnormalStep(s Snapshot) Step {
	threshold := fmt.Sprintf("%d event(s) of one kind in %d cycle(s) is a SHARED FAULT", s.Settings.AbnormalMinEvents, s.Settings.AbnormalMinCycles)
	if !s.Abnormal.Known {
		return notGathered("A4", threshold)
	}
	tallies := tallyKinds(s.Abnormal.Value, s.Now)
	if len(tallies) == 0 {
		return Step{ID: "A4", Reading: "no WARN or ERROR event", Threshold: threshold, Result: ResultOK}
	}
	top := tallies[0]
	st := Step{ID: "A4", Threshold: threshold, Result: ResultOK, Reading: fmt.Sprintf("%s: %d event(s) in %d cycle(s)", top.kind, top.events, len(top.cycles))}
	if top.events >= s.Settings.AbnormalMinEvents && len(top.cycles) >= s.Settings.AbnormalMinCycles {
		st.Reading += "; SHARED FAULT"
		st.Result, st.Action = ResultBrake, brakeAction
	}
	return st
}

func tallyKinds(events []CycleEvent, now time.Time) []*kindTally {
	byKind := map[string]*kindTally{}
	for _, e := range events {
		if !countsAsAbnormal(e.Event, now) {
			continue
		}
		kind := string(e.Event.EventType)
		t := byKind[kind]
		if t == nil {
			t = &kindTally{kind: kind, cycles: map[int]bool{}}
			byKind[kind] = t
		}
		t.events++
		t.cycles[e.Cycle] = true
	}
	out := make([]*kindTally, 0, len(byKind))
	for _, t := range byKind {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].events != out[j].events {
			return out[i].events > out[j].events
		}
		return out[i].kind < out[j].kind
	})
	return out
}

func countsAsAbnormal(e dispatchevents.Event, now time.Time) bool {
	if e.Severity != dispatchevents.SeverityWarn && e.Severity != dispatchevents.SeverityError {
		return false
	}
	at, err := time.Parse(time.RFC3339, e.Timestamp)
	return err != nil || !at.After(now)
}
