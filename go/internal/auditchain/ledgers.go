package auditchain

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"
)

const outcomePass = string(handoff.ExitPass)

type PairEval struct {
	Pair          string
	RequestAbsent bool
	LedgerAbsent  bool
	Outcome       string
	Unmet         []string
	Blocked       []string
	Errors        []string
	Request       handoff.Request
	Ledger        handoff.Ledger
}

type LedgerInputs struct {
	Pairs []PairEval
}

type LedgerFinding struct {
	Link    LinkID `json:"link"`
	Pair    string `json:"pair"`
	Ask     string `json:"ask,omitempty"`
	Message string `json:"message"`
}

func pairOf(in LedgerInputs, name string) PairEval {
	for _, p := range in.Pairs {
		if p.Pair == name {
			return p
		}
	}
	return PairEval{Pair: name}
}

func TDDSkipped(in LedgerInputs) bool {
	return pairOf(in, request.PairTDDBuild).RequestAbsent
}

func TriageSkipped(in LedgerInputs) bool {
	return pairOf(in, request.PairTriageTDD).RequestAbsent
}

func WalkLedgers(in LedgerInputs) []LedgerFinding {
	if TDDSkipped(in) {
		return nil
	}
	triage, build := pairOf(in, request.PairTriageTDD), pairOf(in, request.PairTDDBuild)
	var findings []LedgerFinding
	for _, p := range []struct {
		eval PairEval
		link LinkID
	}{{triage, LinkSelection}, {build, LinkImplementation}} {
		if p.eval.RequestAbsent {
			continue
		}
		if f, bad := pairFinding(p.eval, p.link); bad {
			findings = append(findings, f)
		}
	}
	if !triage.RequestAbsent && triage.Outcome == outcomePass && build.Outcome == outcomePass {
		findings = append(findings, validationFindings(triage, build)...)
	}
	return findings
}

func pairFinding(p PairEval, link LinkID) (LedgerFinding, bool) {
	switch {
	case p.LedgerAbsent:
		return LedgerFinding{Link: link, Pair: p.Pair, Message: fmt.Sprintf("handoff %s: the ledger is missing", p.Pair)}, true
	case p.Outcome != outcomePass:
		msg := fmt.Sprintf("handoff %s: outcome %s; unmet asks: %s; blocked asks: %s; errors: %s",
			p.Pair, p.Outcome, strings.Join(p.Unmet, ", "), strings.Join(p.Blocked, ", "), strings.Join(p.Errors, "; "))
		return LedgerFinding{Link: link, Pair: p.Pair, Ask: strings.Join(p.Unmet, ", "), Message: msg}, true
	}
	return LedgerFinding{}, false
}

func validationFindings(triage, build PairEval) []LedgerFinding {
	var findings []LedgerFinding
	for _, a := range triage.Request.Asks {
		if !acceptanceReachesAMetBuildEntry(a.ID, triage, build) {
			findings = append(findings, LedgerFinding{
				Link: LinkSpecification, Pair: request.PairTriageTDD, Ask: a.ID,
				Message: fmt.Sprintf("inbox acceptance %s maps to no met entry of the %s ledger", a.ID, build.Pair),
			})
		}
	}
	return findings
}

func acceptanceReachesAMetBuildEntry(askID string, triage, build PairEval) bool {
	entry, ok := metEntryOf(triage.Ledger, askID)
	if !ok {
		return false
	}
	for _, ev := range entry.Evidence {
		if ev.Kind != "predicate" {
			continue
		}
		name := evidence.PredicateName(ev.Ref)
		if slices.ContainsFunc(build.Request.Asks, func(a handoff.Ask) bool { return a.ID == name }) {
			if _, met := metEntryOf(build.Ledger, name); met {
				return true
			}
		}
	}
	return false
}

func metEntryOf(l handoff.Ledger, askID string) (handoff.Entry, bool) {
	for _, e := range l.Entries {
		if e.Ask == askID && e.Status == handoff.StatusMet {
			return e, true
		}
	}
	return handoff.Entry{}, false
}
