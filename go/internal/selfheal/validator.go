package selfheal

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	DecisionSchema   = "evolve.healing.decision/1"
	VerdictAccepted  = "accepted"
	VerdictRefused   = "refused"
	convergeFailures = 2
)

var ErrNoAcceptedProposal = errors.New("selfheal: no accepted proposal")

type Repair struct {
	IncidentID string    `json:"incident_id"`
	Class      Class     `json:"class"`
	Failed     bool      `json:"failed"`
	At         time.Time `json:"at"`
}

type History struct {
	Decisions []Decision
	Repairs   []Repair
}

type Limits struct {
	Window          time.Duration
	Repairs         map[string]int
	BreakerFailures int
	BreakerOpen     time.Duration
}

type Validator struct {
	Inventory Inventory
	History   History
	Limits    Limits
	Stage     string
}

type Decision struct {
	Schema       string                         `json:"schema"`
	IncidentID   string                         `json:"incident_id"`
	Class        Class                          `json:"class"`
	Tier         string                         `json:"tier"`
	Action       *phasecontract.DiagnosisAction `json:"action,omitempty"`
	Verdict      string                         `json:"verdict"`
	Precondition string                         `json:"precondition,omitempty"`
	Reason       string                         `json:"reason,omitempty"`
	Escalated    bool                           `json:"escalated"`
	Stage        string                         `json:"stage"`
	At           time.Time                      `json:"at"`
}

func (v Validator) Decide(inc Incident, a phasecontract.DiagnosisAction, now time.Time) Decision {
	d := v.decision(inc, now)
	d.Action = &a
	entry, ok := catalogEntry(a.Kind)
	if !ok {
		return v.refused(d, PreCatalog, fmt.Sprintf("the action %q is not in the catalog %v", a.Kind, CatalogActions()))
	}
	if pre, reason := entry.firstRefusal(check{inc: inc, params: a.Params, inv: v.Inventory, history: v.History}); pre != "" {
		return v.refused(d, pre, reason)
	}
	if pre, reason := v.repairControls(entry, inc, now); pre != "" {
		return v.refused(d, pre, reason)
	}
	d.Verdict, d.Escalated = VerdictAccepted, a.Kind == phasecontract.ActionHaltAndEscalate
	return d
}

func (v Validator) Refuse(inc Incident, err error, now time.Time) Decision {
	return v.refused(v.decision(inc, now), PreDiagnosis, err.Error())
}

func (v Validator) Record(d Decision) Validator {
	v.History.Decisions = append(slices.Clone(v.History.Decisions), d)
	return v
}

func (v Validator) decision(inc Incident, now time.Time) Decision {
	return Decision{Schema: DecisionSchema, IncidentID: inc.ID, Class: inc.Class, Tier: inc.Tier, Stage: v.Stage, At: now}
}

func (v Validator) refused(d Decision, pre, reason string) Decision {
	d.Verdict, d.Precondition, d.Reason = VerdictRefused, pre, reason
	d.Escalated = pre == PreBreaker || pre == PreConverge || slices.ContainsFunc(v.History.Decisions, func(old Decision) bool {
		return old.IncidentID == d.IncidentID && old.Verdict == VerdictRefused
	})
	return d
}

func (v Validator) repairControls(entry catalogAction, inc Incident, now time.Time) (string, string) {
	if !entry.repair || inc.Tier == TierKernel {
		return "", ""
	}
	budget := v.Limits.Repairs[inc.Tier]
	if failed := v.failedRepairs(inc.ID); failed >= convergeFailures {
		return PreConverge, fmt.Sprintf("%d repairs of the incident failed; it goes to the operator", failed)
	}
	if reason := v.breaker(inc.Class, now); reason != "" {
		return PreBreaker, reason
	}
	if spent := v.spentRepairs(inc.Tier, now.Add(-v.Limits.Window)); spent >= budget {
		return PreBudget, fmt.Sprintf("the %s budget of %d repairs in %s is spent", inc.Tier, budget, v.Limits.Window)
	}
	return "", ""
}

func (v Validator) failedRepairs(incidentID string) int {
	n := 0
	for _, r := range v.History.Repairs {
		if r.IncidentID == incidentID && r.Failed {
			n++
		}
	}
	return n
}

func (v Validator) breaker(class Class, now time.Time) string {
	openedAt, open := v.breakerOpenedAt(class)
	if !open {
		return ""
	}
	halfOpen := openedAt.Add(v.Limits.BreakerOpen)
	if now.Before(halfOpen) {
		return fmt.Sprintf("the breaker of the class %s is open until %s", class, halfOpen.Format(time.RFC3339))
	}
	if v.acceptedRepairs(func(d Decision) bool { return d.Class == class && !d.At.Before(halfOpen) }) > 0 {
		return fmt.Sprintf("the breaker of the class %s is half-open, and its one repair already ran", class)
	}
	return ""
}

func (v Validator) breakerOpenedAt(class Class) (time.Time, bool) {
	repairs := slices.Clone(v.History.Repairs)
	slices.SortStableFunc(repairs, func(a, b Repair) int { return a.At.Compare(b.At) })
	var openedAt time.Time
	open, failures := false, 0
	for _, r := range repairs {
		switch {
		case r.Class != class:
		case open && r.At.Before(openedAt.Add(v.Limits.BreakerOpen)):
		case open && r.Failed:
			openedAt = r.At
		case open:
			open, failures = false, 0
		case r.Failed:
			failures++
			if failures >= v.Limits.BreakerFailures {
				open, openedAt = true, r.At
			}
		default:
			failures = 0
		}
	}
	return openedAt, open
}

func (v Validator) spentRepairs(tier string, since time.Time) int {
	return v.acceptedRepairs(func(d Decision) bool { return d.Tier == tier && d.At.After(since) })
}

func (v Validator) acceptedRepairs(match func(Decision) bool) int {
	n := 0
	for _, d := range v.History.Decisions {
		if d.Verdict != VerdictAccepted || d.Action == nil || !match(d) {
			continue
		}
		if entry, ok := catalogEntry(d.Action.Kind); ok && entry.repair {
			n++
		}
	}
	return n
}
