package wave

import (
	"fmt"
	"sort"
	"strings"
)

type EventKind string

const (
	EventPhase         EventKind = "phase"
	EventSealed        EventKind = "sealed"
	EventShipped       EventKind = "shipped"
	EventQuotaPaused   EventKind = "quota-paused"
	EventLoginRequired EventKind = "login-required"
	EventLoopExit      EventKind = "loop-exit"
	EventWaveStarted   EventKind = "wave-started"
	EventPool          EventKind = "pool"
	EventStopped       EventKind = "stopped"
	EventStall         EventKind = "stall"
)

type Snapshot struct {
	Cycles   []Cycle
	LoopLive bool
}

type Event struct {
	Kind     EventKind `json:"kind"`
	Cycle    int       `json:"cycle,omitempty"`
	Detail   string    `json:"detail,omitempty"`
	Trigger  string    `json:"trigger,omitempty"`
	Lanes    []string  `json:"lanes,omitempty"`
	Evidence []string  `json:"evidence,omitempty"`
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
	case EventLoginRequired:
		return fmt.Sprintf("cycle %d: login-required in %s", e.Cycle, e.Detail)
	case EventPool:
		return e.Detail
	case EventStopped:
		return fmt.Sprintf("cycle %d: %s", e.Cycle, e.Detail)
	case EventStall:
		return formatStallEvent(e)
	default:
		return fmt.Sprintf("cycle %d: %s %s", e.Cycle, e.Kind, e.Detail)
	}
}

func formatStallEvent(e Event) string {
	text := "stall"
	if e.Trigger != "" {
		text += " " + e.Trigger
	}
	if e.Detail != "" {
		text += ": " + e.Detail
	}
	if len(e.Lanes) > 0 {
		text += "; lanes " + strings.Join(e.Lanes, ", ")
	}
	if len(e.Evidence) > 0 {
		text += "; evidence " + strings.Join(e.Evidence, ", ")
	}
	return text
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
	if c.LoginWalls > old.LoginWalls {
		events = append(events, Event{Kind: EventLoginRequired, Cycle: c.ID, Detail: c.Phase})
	}
	if c.Verdict == "" && c.Termination != "" && c.Termination != old.Termination {
		events = append(events, Event{Kind: EventStopped, Cycle: c.ID, Detail: c.Termination})
	}
	if c.Verdict != "" && c.Verdict != old.Verdict {
		events = append(events, Event{Kind: EventSealed, Cycle: c.ID, Detail: c.Verdict})
	}
	if c.Shipped && !old.Shipped {
		events = append(events, Event{Kind: EventShipped, Cycle: c.ID})
	}
	return events
}
