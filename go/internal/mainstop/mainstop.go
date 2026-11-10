// Package mainstop holds the record of a red main and the pure transitions of its stop.
// See docs/architecture/packages/internal-mainstop.md.
package mainstop

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
)

const (
	Schema        = "main-red-stop/v1"
	fixItemPrefix = "fix-main-red-"
	fixItemSHALen = 8
)

type Parked struct {
	Cycle     int      `json:"cycle"`
	Scope     []string `json:"scope"`
	Workspace string   `json:"workspace"`
}

type Stop struct {
	Schema     string         `json:"schema"`
	Keys       []baseattr.Key `json:"keys"`
	MainSHA    string         `json:"main_sha"`
	OpenedAt   time.Time      `json:"opened_at"`
	FixItem    string         `json:"fix_item"`
	Parked     []Parked       `json:"parked"`
	Escalated  *time.Time     `json:"escalated_at,omitempty"`
	ReleasedAt *time.Time     `json:"released_at,omitempty"`
	ReleasedBy string         `json:"released_by,omitempty"`
}

type Proof struct {
	Base     string
	Green    []baseattr.Key
	LaneOnly []baseattr.Key
}

type Admission int

const (
	AdmitNoStop Admission = iota
	AdmitFix
	RefuseRebase
	RefuseNotAFix
)

func FixItemID(mainSHA string) string {
	return fixItemPrefix + mainSHA[:min(len(mainSHA), fixItemSHALen)]
}

func (s Stop) Holds() bool {
	return len(s.Keys) > 0 && s.ReleasedAt == nil
}

func (s Stop) Opened(v baseattr.Verdict, fixItem string, now time.Time) Stop {
	if !s.Holds() {
		return Stop{Schema: Schema, Keys: slices.Clone(v.MainRed), MainSHA: v.Main, OpenedAt: now, FixItem: fixItem}
	}
	next := s
	next.Keys = slices.Clone(s.Keys)
	for _, k := range v.MainRed {
		if !slices.Contains(next.Keys, k) {
			next.Keys = append(next.Keys, k)
		}
	}
	if v.Main != "" {
		next.MainSHA = v.Main
	}
	return next
}

func (s Stop) WithParked(p Parked) Stop {
	next := s
	next.Parked = slices.DeleteFunc(slices.Clone(s.Parked), func(old Parked) bool { return old.Cycle == p.Cycle })
	next.Parked = append(next.Parked, p)
	slices.SortFunc(next.Parked, func(a, b Parked) int { return cmp.Compare(a.Cycle, b.Cycle) })
	return next
}

func (s Stop) Admit(mainSHA string, p Proof) Admission {
	if !s.Holds() {
		return AdmitNoStop
	}
	if mainSHA == "" || p.Base != mainSHA {
		return RefuseRebase
	}
	if len(p.LaneOnly) == 0 && slices.ContainsFunc(s.Keys, func(k baseattr.Key) bool { return slices.Contains(p.Green, k) }) {
		return AdmitFix
	}
	return RefuseNotAFix
}

func (s Stop) Released(green []baseattr.Key, by string, now time.Time) Stop {
	if !s.Holds() {
		return s
	}
	next := s
	next.Keys = slices.DeleteFunc(slices.Clone(s.Keys), func(k baseattr.Key) bool { return slices.Contains(green, k) })
	if len(next.Keys) == 0 {
		next.ReleasedAt = &now
		next.ReleasedBy = by
	}
	return next
}

func (s Stop) Overdue(now time.Time, bound time.Duration) bool {
	return s.Holds() && bound > 0 && now.Sub(s.OpenedAt) > bound
}

func (s Stop) Escalate(now time.Time) Stop {
	if s.Escalated != nil {
		return s
	}
	next := s
	next.Escalated = &now
	return next
}

func (s Stop) Cause() string {
	return fmt.Sprintf("main is red at %s since %s: %s; parked cycles: %s",
		s.MainSHA, s.OpenedAt.UTC().Format(time.RFC3339), baseattr.Names(s.Keys), s.parkedText())
}

func (s Stop) parkedText() string {
	if len(s.Parked) == 0 {
		return "none"
	}
	return s.ParkedCycles()
}

func (s Stop) ParkedCycles() string {
	cycles := make([]string, len(s.Parked))
	for i, p := range s.Parked {
		cycles[i] = fmt.Sprint(p.Cycle)
	}
	return strings.Join(cycles, ", ")
}

func (s Stop) ResumeCondition() string {
	return "a landing turns each key green on main, or a rerun finds them green on the main tip; the parked lanes then resume in the next pool fill"
}
