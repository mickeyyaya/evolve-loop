// Package auditchain derives the audit verdict from the auditor's chain of
// stage-to-stage coherence findings, never from the auditor's own assertion.
// See docs/architecture/packages/internal-auditchain.md.
package auditchain

import (
	"fmt"
	"sort"
	"strings"
)

type LinkID string

const (
	LinkIntentFidelity LinkID = "intent-fidelity"
	LinkSelection      LinkID = "selection-fidelity"
	LinkSpecification  LinkID = "specification-fidelity"
	LinkImplementation LinkID = "implementation-fidelity"
	LinkNarrative      LinkID = "narrative-fidelity"
	LinkDelivery       LinkID = "delivery-fidelity"
	LinkEvidence       LinkID = "evidence-fidelity"
)

func RequiredLinks() []LinkID {
	return []LinkID{
		LinkIntentFidelity,
		LinkSelection,
		LinkSpecification,
		LinkImplementation,
		LinkNarrative,
		LinkDelivery,
		LinkEvidence,
	}
}

type Status string

const (
	StatusCoherent     Status = "coherent"
	StatusIncoherent   Status = "incoherent"
	StatusUnverifiable Status = "unverifiable"
)

type Link struct {
	ID       LinkID `json:"id"`
	Status   Status `json:"status"`
	Finding  string `json:"finding"`
	Citation string `json:"citation"`
}

type Chain []Link

type Verdict string

const (
	VerdictPASS Verdict = "PASS"
	VerdictWARN Verdict = "WARN"
	VerdictFAIL Verdict = "FAIL"
)

type Conclusion struct {
	Verdict   Verdict  `json:"verdict"`
	Rationale string   `json:"rationale"`
	Diagnoses []string `json:"diagnoses,omitempty"`
}

func Conclude(c Chain) Conclusion {
	present := map[LinkID]Link{}
	for _, l := range c {
		present[l.ID] = l
	}
	var missing, incoherent, unverifiable []string
	for _, id := range RequiredLinks() {
		l, ok := present[id]
		if !ok {
			missing = append(missing, string(id))
			continue
		}
		switch l.Status {
		case StatusIncoherent:
			incoherent = append(incoherent, fmt.Sprintf("%s (%s)", id, l.Finding))
		case StatusUnverifiable:
			unverifiable = append(unverifiable, fmt.Sprintf("%s (%s)", id, l.Finding))
		}
	}
	diag := Diagnose(c)
	switch {
	case len(missing) > 0:
		// A missing link is FAIL, never WARN: omission must not be gentler than an honest incoherent answer.
		return Conclusion{Verdict: VerdictFAIL, Diagnoses: diag,
			Rationale: "the chain is incomplete — missing link(s): " + strings.Join(missing, ", ") +
				". A relationship nobody reported is not a relationship nobody needed to check."}
	case len(incoherent) > 0:
		return Conclusion{Verdict: VerdictFAIL, Diagnoses: diag,
			Rationale: "incoherent link(s): " + strings.Join(incoherent, "; ")}
	case len(unverifiable) > 0:
		return Conclusion{Verdict: VerdictWARN, Diagnoses: diag,
			Rationale: "unverifiable link(s): " + strings.Join(unverifiable, "; ") +
				". Not established, therefore not asserted — resolve them or accept a qualified verdict."}
	}
	return Conclusion{Verdict: VerdictPASS, Diagnoses: diag,
		Rationale: "every required relationship was examined and holds, each against a citation a third party can check"}
}

func Validate(c Chain) []error {
	var errs []error
	known := map[LinkID]bool{}
	for _, id := range RequiredLinks() {
		known[id] = true
	}
	seen := map[LinkID]bool{}
	citations := map[string]int{}
	for _, l := range c {
		if !known[l.ID] {
			errs = append(errs, fmt.Errorf("auditchain: unknown link %q — the chain's shape is the contract; a link nobody defined is a finding nobody can check", l.ID))
			continue
		}
		if seen[l.ID] {
			errs = append(errs, fmt.Errorf("auditchain: %s reported twice — one relationship carries one status, or the chain can hold both an answer and its opposite", l.ID))
		}
		seen[l.ID] = true
		if strings.TrimSpace(l.Citation) == "" {
			errs = append(errs, fmt.Errorf("auditchain: %s has no citation — a finding a third party cannot go and look at is the auditor's opinion wearing the shape of one", l.ID))
		}
		if strings.TrimSpace(l.Finding) == "" {
			errs = append(errs, fmt.Errorf("auditchain: %s has no finding — a status with no reasoning is a vote, not a review", l.ID))
		}
		citations[artifactOf(l.Citation)]++
	}
	if len(c) > 1 && len(citations) == 1 {
		for a := range citations {
			errs = append(errs, fmt.Errorf("auditchain: every link cites the same single artifact (%s) — the chain spans stages, so its citations must too; this is the shape of a chain that was narrated rather than walked", a))
		}
	}
	return errs
}

func artifactOf(citation string) string {
	if i := strings.IndexByte(citation, ':'); i > 0 {
		return citation[:i]
	}
	return citation
}

func Diagnose(c Chain) []string {
	st := map[LinkID]Status{}
	for _, l := range c {
		st[l.ID] = l.Status
	}
	var out []string
	if st[LinkDelivery] == StatusIncoherent {
		out = append(out, "derailed: the change is coherent in itself and delivers something other than the intent")
	}
	if st[LinkNarrative] == StatusIncoherent {
		out = append(out, "specious: the report claims more than the bytes do")
	}
	if st[LinkSpecification] == StatusIncoherent && st[LinkImplementation] == StatusCoherent {
		out = append(out, "paradoxical: the implementation satisfies tests that no longer encode the acceptance criteria — the specification moved to meet the code")
	}
	if st[LinkEvidence] == StatusIncoherent {
		out = append(out, "deceptive: the cited evidence was produced by the party being judged rather than by running the gate")
	}
	sort.Strings(out)
	return out
}

func withLink(c Chain, id LinkID, st Status, finding string) Chain {
	out := make(Chain, len(c))
	copy(out, c)
	for i := range out {
		if out[i].ID == id {
			out[i].Status = st
			out[i].Finding = finding
			return out
		}
	}
	return append(out, Link{ID: id, Status: st, Finding: finding, Citation: "unset"})
}
