package core

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

const (
	stopReasonLogin   = "login-required"
	stopReasonQuota   = "quota-pause"
	stopReasonUnknown = "wall-unknown"
	wallTimeLayout    = "2006-01-02T15:04Z07:00"
	noWallRecord      = "no cause was recorded for this pause"
)

type FamilyEnd = cyclestate.WallEnd

type Wall struct {
	Ends []FamilyEnd `json:"ends,omitempty"`
}

func endRank(pattern string) int {
	switch {
	case clihealth.LoginPattern(pattern):
		return 3
	case clihealth.QuotaPattern(pattern):
		return 2
	case pattern != "":
		return 1
	}
	return 0
}

func (w Wall) positions() map[string]int {
	at := make(map[string]int, len(w.Ends))
	for i, e := range w.Ends {
		if _, seen := at[e.CLI]; !seen {
			at[e.CLI] = i
		}
	}
	return at
}

func (w Wall) Observe(end FamilyEnd) Wall {
	end.CLI = llmroute.Family(end.CLI)
	out := Wall{Ends: slices.Clone(w.Ends)}
	i, found := out.positions()[end.CLI]
	if !found {
		out.Ends = append(out.Ends, end)
		return out
	}
	prev := out.Ends[i]
	if endRank(end.Pattern) < endRank(prev.Pattern) {
		return out
	}
	if end.ResetAt.IsZero() && endRank(end.Pattern) == endRank(prev.Pattern) {
		end.ResetAt = prev.ResetAt
	}
	out.Ends[i] = end
	return out
}

func (w Wall) Admits(end FamilyEnd) bool {
	family := llmroute.Family(end.CLI)
	i, found := w.positions()[family]
	return !found || endRank(end.Pattern) >= endRank(w.Ends[i].Pattern)
}

func (w Wall) Merge(other Wall) Wall {
	out := w
	for _, e := range other.Ends {
		out = out.Observe(e)
	}
	return out
}

func (w Wall) Families() []string {
	families := make([]string, 0, len(w.Ends))
	for _, e := range w.Ends {
		families = append(families, e.CLI)
	}
	return families
}

func (w Wall) HasLogin() bool {
	return slices.ContainsFunc(w.Ends, func(e FamilyEnd) bool { return clihealth.LoginPattern(e.Pattern) })
}

func (w Wall) HasQuota() bool {
	return slices.ContainsFunc(w.Ends, func(e FamilyEnd) bool { return clihealth.QuotaPattern(e.Pattern) })
}

func (w Wall) CountsAsQuotaPause() bool {
	if w.HasQuota() {
		return true
	}
	return !slices.ContainsFunc(w.Ends, func(e FamilyEnd) bool { return e.Pattern != "" })
}

func (w Wall) StopReason() string {
	switch {
	case w.HasLogin():
		return stopReasonLogin
	case len(w.Ends) > 0 && !slices.ContainsFunc(w.Ends, func(e FamilyEnd) bool { return !clihealth.QuotaPattern(e.Pattern) }):
		return stopReasonQuota
	}
	return stopReasonUnknown
}

func (w Wall) StopReasonFor(timed bool) string {
	if len(w.Ends) == 0 && timed {
		return stopReasonQuota
	}
	return w.StopReason()
}

func endClause(e FamilyEnd) string {
	switch {
	case clihealth.LoginPattern(e.Pattern):
		return e.CLI + " needs a login"
	case clihealth.QuotaPattern(e.Pattern):
		if e.ResetAt.IsZero() {
			return e.CLI + " is on quota, reset unknown"
		}
		return e.CLI + " is on quota until " + e.ResetAt.Format(wallTimeLayout)
	case e.Pattern != "":
		return e.CLI + " ended on " + e.Pattern
	}
	return e.CLI + " ended with no recorded cause"
}

func (w Wall) Clauses() string {
	if len(w.Ends) == 0 {
		return noWallRecord
	}
	clauses := make([]string, 0, len(w.Ends))
	for _, e := range w.Ends {
		clauses = append(clauses, endClause(e))
	}
	return strings.Join(clauses, "; ")
}

func endAction(e FamilyEnd) string {
	switch {
	case clihealth.LoginPattern(e.Pattern):
		return clihealth.LoginCommand(e.CLI)
	case clihealth.QuotaPattern(e.Pattern):
		return ""
	case e.Evidence != "":
		return "read " + e.Evidence
	}
	return ""
}

func endWait(e FamilyEnd) string {
	if !clihealth.QuotaPattern(e.Pattern) {
		return ""
	}
	if e.ResetAt.IsZero() {
		return "wait for " + e.CLI + ", reset unknown"
	}
	return "wait for " + e.CLI + " at " + e.ResetAt.Format(wallTimeLayout)
}

func WallNextStep(wall Wall, workspace string) string {
	var actions, waits []string
	for _, e := range wall.Ends {
		if a := endAction(e); a != "" {
			actions = append(actions, a)
		}
		if w := endWait(e); w != "" {
			waits = append(waits, w)
		}
	}
	resume := "evolve loop --resume."
	if len(LaneScopeIDs(workspace)) > 0 {
		resume = "evolve cycle resume --all, then evolve wave next."
	}
	if len(actions) == 0 && len(waits) == 0 {
		return resume
	}
	return strings.Join(append(actions, waits...), ", or ") + ". Then " + resume
}
