package auditchain

// evidence_access.go — the precondition the chain rests on: a phase that JUDGES
// must be handed what the phases before it produced.
//
// This is not a convenience. Each of the four failures the chain exists to
// catch is measured against an artifact the judge did not write:
//
//	derailed    is measured against intent.md
//	specious    is measured against build-report.md
//	paradoxical is measured against the tests AND the criteria they should encode
//	deceptive   is measured against the gate outputs
//
// Withholding those does not make the review stricter — it makes it guess, and
// a reviewer that must produce a verdict from a guess produces PASS. The
// pipeline has the scar to prove the general shape: a truncated audit prompt
// cost 15 of 30 cycles before anyone noticed the auditor had never been shown
// the rules it was being held to.
//
// So entitlement is a CONTRACT. Every judging phase declares what it must be
// able to read; a dispatch that omits one is a pipeline defect rather than a
// thinner prompt; and — the part that gives it teeth — a link whose evidence
// was never supplied cannot be reported coherent, so the gap reaches the
// verdict instead of sitting in a log nobody reads.

import (
	"fmt"
	"sort"
	"strings"
)

// judgingPhases are the phases whose OUTPUT is a judgement about work someone
// else did. Entitlement follows the act of deciding, not seniority: a producing
// phase handed the whole prior corpus is cost with no decision attached, and
// prompts that grow without bound are how the truncation incident happened.
var judgingPhases = map[string]bool{
	"audit":                      true,
	"adversarial-review":         true,
	"coverage-gate":              true,
	"plan-review":                true,
	"inherited-defect-reconcile": true,
	"retrospective":              true,
	"retro":                      true,
}

// IsJudging reports whether a phase rules on another phase's work.
func IsJudging(phase string) bool { return judgingPhases[strings.ToLower(strings.TrimSpace(phase))] }

// linkEvidence maps each link to the artifacts it is read FROM. It is the
// bridge between the two halves of the design: a chain is only walkable if
// every required link has something behind it, and this table is what makes
// that checkable rather than hoped for.
var linkEvidence = map[LinkID][]string{
	LinkIntentFidelity: {"intent.md", "scout-report.md"},
	LinkSelection:      {"triage-decision.json", "intent.md"},
	LinkSpecification:  {"intent.md", "covering-tests.md"},
	LinkImplementation: {"covering-tests.md", "build-report.md"},
	LinkNarrative:      {"build-report.md"},
	LinkDelivery:       {"intent.md", "build-report.md"},
	LinkEvidence:       {"acs-verdict.json", "coverage-gate-report.md"},
}

// EvidenceFor returns the artifacts a link is read from.
func EvidenceFor(id LinkID) ([]string, bool) {
	srcs, ok := linkEvidence[id]
	return srcs, ok
}

// RequiredEvidence is the artifact set a judging phase must be able to read —
// the union of what its links are measured against. Empty for producing
// phases: the entitlement must not degrade into "everyone reads everything".
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

// MissingEvidence reports what a judging phase was entitled to and not given.
//
// Named, not counted: the failure a missing artifact causes appears later and
// somewhere else — as an unverifiable link if the auditor is honest, and as an
// asserted one if it is not — so the gap has to be reported where it is still
// attributable.
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
		if !have[r] {
			missing = append(missing, r)
		}
	}
	return missing
}

// ConcludeWithEvidence derives the verdict AND accounts for what the judge was
// actually able to read.
//
// The teeth of the entitlement. Without this, a phase dispatched without the
// intent could still report delivery-fidelity "coherent" and PASS — the chain
// would be narrated rather than walked, and it would look identical on paper to
// one that was. A link whose evidence was never supplied is therefore
// downgraded to unverifiable no matter what the judge claimed: not an
// accusation, simply the honest status for a relationship nobody was in a
// position to check.
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
	var affected []string
	for i, l := range downgraded {
		for _, src := range linkEvidence[l.ID] {
			if absent[src] && l.Status == StatusCoherent {
				downgraded[i].Status = StatusUnverifiable
				downgraded[i].Finding = fmt.Sprintf("reported coherent, but %s was not supplied to this phase — downgraded: %s", src, l.Finding)
				affected = append(affected, string(l.ID))
				break
			}
		}
	}
	out := Conclude(downgraded)
	out.Rationale = fmt.Sprintf("evidence not supplied to %s: %s (links downgraded: %s). %s",
		phase, strings.Join(missing, ", "), strings.Join(affected, ", "), out.Rationale)
	return out
}
