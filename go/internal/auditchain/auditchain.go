// Package auditchain makes the audit verdict the CONCLUSION of a chain of
// reasoning across every prior phase, instead of an assertion attached to one.
//
// THE PROBLEM WITH VERDICT-AS-ASSERTION. An auditor that reads the diff and
// declares PASS is doing the one thing no reviewer should: producing a judgement
// whose support is invisible. Everything downstream then has to either trust it
// or re-derive it, and the failures that matter most are exactly the ones a
// diff-only reading cannot see. A human reviewer catches those instantly, and
// not by being cleverer — by holding the WHOLE chain at once: what was asked,
// what was planned, what the tests demanded, what the builder claimed, what the
// bytes do, what the gates actually ran.
//
// THE INSIGHT THIS PACKAGE ENCODES. Derailed, specious, paradoxical and
// deceptive are not properties of a diff. They are INCOHERENCES BETWEEN STAGES:
//
//	derailed    — the change is internally consistent and delivers something
//	              other than what was asked (intent ↮ diff).
//	specious    — the narrative is larger than the bytes (diff ↮ build report).
//	paradoxical — the implementation satisfies the tests because the TESTS were
//	              moved to it. Each link reads fine alone; together they
//	              contradict (intent ↮ tdd, while tdd ↔ diff is "coherent").
//	deceptive   — the evidence was produced by the party being judged
//	              (diff ↮ gates).
//
// None of those is visible to a check that reads one artifact. All of them are
// obvious the moment the relationships are examined explicitly — which is what
// a chain is.
//
// THE POLICY. The auditor's obligation is to produce the CHAIN. The verdict is
// then a function of it:
//
//	all links coherent          → PASS
//	any link incoherent         → FAIL, naming the link
//	any link unverifiable       → WARN — "I could not check this" is not
//	                              evidence of coherence, and must never be
//	                              launderable into it
//	any required link missing   → cannot conclude — silence about a relationship
//	                              is the cheapest way to avoid reporting it, so
//	                              absence is louder than a negative finding
//
// This puts judgement where judgement belongs (is THIS link coherent?) and
// determinism where that belongs (given these statuses, what follows?). An
// auditor cannot assert PASS over an incoherent link, because the verdict is
// not the auditor's to assert — a property no amount of persona wording can
// achieve, and the reason this is code rather than prose in a prompt.
//
// AND WHY IT BEATS A CHECKLIST OF PROXIES. Mechanical anti-gaming rules —
// "was this path a test?", "did the effect loosen?" — are proxies, and this
// repo's scars are all proxies used as verdicts (a substring match that
// force-FAILed four batches; a scope rule that discarded a working
// implementation four times). A chain does not proxy anything: it asks the
// reviewer's own question, one relationship at a time, and records the answer
// with the citation that supports it.
package auditchain

import (
	"fmt"
	"sort"
	"strings"
)

// LinkID names one coherence check between two stages of the cycle. The set is
// closed: the chain's SHAPE is the contract, so an auditor cannot invent a link
// to report on, nor quietly drop one it would rather not answer.
type LinkID string

const (
	// LinkIntentFidelity — does the intent capture the queued item, or has the
	// task been restated into something easier?
	LinkIntentFidelity LinkID = "intent-fidelity"
	// LinkSelection — is the work triage committed to the work the intent
	// describes?
	LinkSelection LinkID = "selection-fidelity"
	// LinkSpecification — do the tests ENCODE the acceptance criteria, or
	// something weaker? Half of "paradoxical" lives here.
	LinkSpecification LinkID = "specification-fidelity"
	// LinkImplementation — does the code satisfy the tests as they now stand?
	LinkImplementation LinkID = "implementation-fidelity"
	// LinkNarrative — does the build report describe what the bytes do?
	// "Specious" lives here.
	LinkNarrative LinkID = "narrative-fidelity"
	// LinkDelivery — does the change deliver the INTENT, not merely something
	// coherent? "Derailed" lives here.
	LinkDelivery LinkID = "delivery-fidelity"
	// LinkEvidence — were the gate results actually produced by running the
	// gates, over these bytes? "Deceptive" lives here.
	LinkEvidence LinkID = "evidence-fidelity"
)

// RequiredLinks is the chain every audit must walk, in reasoning order. Every
// one of them is a relationship a human reviewer checks without noticing they
// are checking it; writing them down is what makes the check auditable, and
// what makes its ABSENCE detectable.
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

// Status is one link's finding.
type Status string

const (
	// StatusCoherent — the two stages agree, and the citation shows where.
	StatusCoherent Status = "coherent"
	// StatusIncoherent — they disagree. Decisive: one is enough to sink a cycle.
	StatusIncoherent Status = "incoherent"
	// StatusUnverifiable — the auditor could not establish it. The honest
	// middle, and deliberately NOT a defect: an auditor forced to choose
	// between "fine" and "broken" for something it could not see will choose
	// "fine", which is how unverifiability becomes a laundering channel.
	StatusUnverifiable Status = "unverifiable"
)

// Link is one adjudicated relationship between stages.
type Link struct {
	ID     LinkID `json:"id"`
	Status Status `json:"status"`
	// Finding is what the auditor concluded, in its own words. This is the
	// judgement, and it is the one field no machine checks — which is why it is
	// never the thing a verdict is computed from.
	Finding string `json:"finding"`
	// Citation is what a third party can go and look at: a file:line, an
	// artifact path, a command and its output. A link without one is the
	// auditor's opinion wearing the shape of a finding.
	Citation string `json:"citation"`
}

// Chain is the audit's reasoning, in order.
type Chain []Link

// Verdict is the entailed conclusion.
type Verdict string

const (
	VerdictPASS Verdict = "PASS"
	VerdictWARN Verdict = "WARN"
	VerdictFAIL Verdict = "FAIL"
)

// Conclusion is the verdict together with the reasoning that entails it.
type Conclusion struct {
	Verdict   Verdict `json:"verdict"`
	Rationale string  `json:"rationale"`
	// Diagnoses names the human-recognisable failure patterns, if any.
	Diagnoses []string `json:"diagnoses,omitempty"`
}

// Conclude derives the verdict from the chain.
//
// Deliberately total and deliberately dull: every input shape yields a verdict,
// and none of them consult the Finding prose. The auditor supplies statuses and
// citations; the conclusion is arithmetic over them. That separation is the
// whole mechanism — it is what makes "PASS" unassertable when a link says
// otherwise.
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
		// Reported as a FAIL rather than a WARN on purpose: an auditor that
		// omits the link it would have had to answer badly must not land in a
		// gentler bucket than one that answered honestly.
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

// Validate reports the chain's own defects — the ways a chain can be malformed
// independently of what it concludes.
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
	// The design rests on the auditor having actually READ the prior phases. A
	// chain whose citations all point at one artifact is an auditor that read
	// one file and inferred the rest of the chain it never walked.
	if len(c) > 1 && len(citations) == 1 {
		for a := range citations {
			errs = append(errs, fmt.Errorf("auditchain: every link cites the same single artifact (%s) — the chain spans stages, so its citations must too; this is the shape of a chain that was narrated rather than walked", a))
		}
	}
	return errs
}

// artifactOf strips the location from a citation so two lines of one file count
// as one artifact.
func artifactOf(citation string) string {
	if i := strings.IndexByte(citation, ':'); i > 0 {
		return citation[:i]
	}
	return citation
}

// Diagnose names the human-recognisable failure patterns the chain exhibits.
//
// This is the vocabulary a reviewer already uses out loud — "this is derailed",
// "that claim is specious" — made precise enough to be produced mechanically
// from link statuses, so the pattern is reported the same way every time
// instead of depending on whether the reader happened to notice.
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
	// The paradox is the one no per-link check can see: the implementation
	// "satisfies" a specification that no longer encodes what was asked, which
	// happens when the tests were moved to the code instead of the code to the
	// tests. BOTH halves failing is an ordinary double failure, not a paradox —
	// the contradiction is the point.
	if st[LinkSpecification] == StatusIncoherent && st[LinkImplementation] == StatusCoherent {
		out = append(out, "paradoxical: the implementation satisfies tests that no longer encode the acceptance criteria — the specification moved to meet the code")
	}
	if st[LinkEvidence] == StatusIncoherent {
		out = append(out, "deceptive: the cited evidence was produced by the party being judged rather than by running the gate")
	}
	sort.Strings(out)
	return out
}

// setLink returns a copy of c with one link's status and finding replaced —
// the small mutation helper the chain's consumers and tests share, kept here so
// a caller never hand-rolls a partial copy and silently drops a citation.
func setLink(c Chain, id LinkID, st Status, finding string) Chain {
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
