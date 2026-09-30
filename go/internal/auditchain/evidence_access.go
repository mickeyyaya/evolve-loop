package auditchain

import (
	"fmt"
	"sort"
	"strings"
)

var judgingPhases = map[string]bool{
	"audit":                      true,
	"adversarial-review":         true,
	"coverage-gate":              true,
	"plan-review":                true,
	"inherited-defect-reconcile": true,
	"retrospective":              true,
	"retro":                      true,
}

func IsJudging(phase string) bool { return judgingPhases[strings.ToLower(strings.TrimSpace(phase))] }

var linkEvidence = map[LinkID][]string{
	LinkIntentFidelity: {"intent.md", "scout-report.md"},
	LinkSelection:      {"triage-decision.json", "intent.md"},
	LinkSpecification:  {"intent.md", "covering-tests.md"},
	LinkImplementation: {"covering-tests.md", "build-report.md"},
	LinkNarrative:      {"build-report.md"},
	LinkDelivery:       {"intent.md", "build-report.md"},
	LinkEvidence:       {"acs-verdict.json", "coverage-gate-report.md"},
}

var conditionalEvidence = map[string]bool{
	"intent.md": true,
}

func EvidenceFor(id LinkID) ([]string, bool) {
	srcs, ok := linkEvidence[id]
	return srcs, ok
}

func RequiredEvidence(phase string) []string {
	if !IsJudging(phase) {
		return nil
	}
	set := map[string]bool{}
	for _, id := range RequiredLinks() {
		for _, a := range linkEvidence[id] {
			set[a] = true
		}
	}
	out := make([]string, 0, len(set))
	for a := range set {
		out = append(out, a)
	}
	sort.Strings(out) // deterministic: this list goes into a dispatch record
	return out
}

func MissingEvidence(phase string, given []string) []string {
	req := RequiredEvidence(phase)
	if len(req) == 0 {
		return nil
	}
	have := make(map[string]bool, len(given))
	for _, g := range given {
		have[g] = true
	}
	var missing []string
	for _, r := range req {
		if !have[r] && !conditionalEvidence[r] {
			missing = append(missing, r)
		}
	}
	return missing
}

func ConcludeWithEvidence(c Chain, phase string, given []string) Conclusion {
	missing := MissingEvidence(phase, given)
	if len(missing) == 0 {
		return Conclude(c)
	}
	absent := make(map[string]bool, len(missing))
	for _, m := range missing {
		absent[m] = true
	}
	downgraded := make(Chain, len(c))
	copy(downgraded, c)
	var downgradedLinks []string
	for i, l := range downgraded {
		if l.Status != StatusCoherent {
			continue
		}
		for _, src := range linkEvidence[l.ID] {
			isWithheld := absent[src] && !conditionalEvidence[src]
			if !isWithheld {
				continue
			}
			downgraded[i].Status = StatusUnverifiable
			downgraded[i].Finding = fmt.Sprintf("reported coherent, but %s was not supplied to this phase — downgraded: %s", src, l.Finding)
			downgradedLinks = append(downgradedLinks, string(l.ID))
			break
		}
	}
	if len(downgradedLinks) == 0 {
		out := Conclude(c)
		out.Rationale = fmt.Sprintf("evidence not supplied to %s: %s (no link lost every source; conclusion stands). %s",
			phase, strings.Join(missing, ", "), out.Rationale)
		return out
	}
	out := Conclude(downgraded)
	out.Rationale = fmt.Sprintf("evidence not supplied to %s: %s (links downgraded: %s). %s",
		phase, strings.Join(missing, ", "), strings.Join(downgradedLinks, ", "), out.Rationale)
	return out
}
