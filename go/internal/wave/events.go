package wave

import (
	"fmt"
	"sort"
)

type EventKind string

const (
	EventPhase       EventKind = "phase"
	EventSealed      EventKind = "sealed"
	EventShipped     EventKind = "shipped"
	EventQuotaPaused EventKind = "quota-paused"
	EventLoopExit    EventKind = "loop-exit"
	EventWaveStarted EventKind = "wave-started"
)

type Snapshot struct {
	Cycles   []Cycle
	LoopLive bool
}

type Event struct {
	Kind   EventKind `json:"kind"`
	Cycle  int       `json:"cycle,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

func (e Event) String() string {
	switch e.Kind {
	case EventLoopExit:
		return "loop: exit"
	case EventWaveStarted:
		return fmt.Sprintf("wave %s started; watch follows it", e.Detail)
	case EventShipped:
		return fmt.Sprintf("cycle %d: shipped", e.Cycle)
	case EventQuotaPaused:
		return fmt.Sprintf("cycle %d: quota-paused in %s", e.Cycle, e.Detail)
	default:
		return fmt.Sprintf("cycle %d: %s %s", e.Cycle, e.Kind, e.Detail)
	}
}

func Diff(prev, cur Snapshot) []Event {
	before := map[int]Cycle{}
	for _, c := range prev.Cycles {
		before[c.ID] = c
	}
	now := append([]Cycle(nil), cur.Cycles...)
	sort.Slice(now, func(i, j int) bool { return now[i].ID < now[j].ID })
	var events []Event
	for _, c := range now {
		events = append(events, cycleEvents(before[c.ID], c)...)
	}
	if prev.LoopLive && !cur.LoopLive {
		events = append(events, Event{Kind: EventLoopExit})
	}
	return events
}

func cycleEvents(old, c Cycle) []Event {
	var events []Event
	if c.Phase != "" && c.Phase != old.Phase {
		events = append(events, Event{Kind: EventPhase, Cycle: c.ID, Detail: c.Phase})
	}
	if c.QuotaPauses > old.QuotaPauses {
		events = append(events, Event{Kind: EventQuotaPaused, Cycle: c.ID, Detail: c.Phase})
	}
	if c.Verdict != "" && c.Verdict != old.Verdict {
		events = append(events, Event{Kind: EventSealed, Cycle: c.ID, Detail: c.Verdict})
	}
	if c.Shipped && !old.Shipped {
		events = append(events, Event{Kind: EventShipped, Cycle: c.ID})
	}
	return events
}
