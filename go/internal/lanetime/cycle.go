package lanetime

import (
	"strings"
	"time"
)

type Span struct {
	Phase   string
	Start   time.Time
	End     time.Time
	Verdict string
}

type Call struct {
	Phase string
	CLI   string
	Start time.Time
	End   time.Time
}

type Cycle struct {
	ID       int
	Spans    []Span
	Calls    []Call
	Unpaired int
	Open     int
}

func (c Cycle) bounds() (time.Time, time.Time) {
	var first, last time.Time
	for _, s := range c.Spans {
		if first.IsZero() || s.Start.Before(first) {
			first = s.Start
		}
		if s.End.After(last) {
			last = s.End
		}
	}
	return first, last
}

func (c Cycle) Lane() time.Duration {
	first, last := c.bounds()
	return last.Sub(first)
}

func (c Cycle) Passed() bool {
	verdict := ""
	for _, s := range c.Spans {
		if (s.Phase == "audit" || s.Phase == "ship") && s.Verdict != "" {
			verdict = s.Verdict
		}
	}
	return verdict == "PASS"
}

func (c Cycle) BuildFamily() string {
	for _, call := range c.Calls {
		if call.Phase == "build" {
			return baseFamily(call.CLI)
		}
	}
	return ""
}

func baseFamily(cli string) string {
	s := strings.TrimSpace(cli)
	for {
		next := strings.TrimSuffix(strings.TrimSuffix(s, "-tmux"), "-p")
		if next == s {
			return s
		}
		s = next
	}
}

func SpanOf(cycles []Cycle) (time.Time, time.Time) {
	var from, to time.Time
	for _, c := range cycles {
		first, last := c.bounds()
		if from.IsZero() || first.Before(from) {
			from = first
		}
		if last.After(to) {
			to = last
		}
	}
	return from, to
}
