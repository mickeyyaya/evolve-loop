package auditchain

// shadow.go — the rollout stage, and the only thing a wave can actually learn
// from.
//
// ADR-0088 says shadow first, and this is what that means concretely: the chain
// is parsed, concluded against the evidence the judging phase was actually
// given, and written beside the cycle — while the phase's own verdict is
// byte-identical to what it would have been with none of this wired.
//
// The datum the soak collects is the DISAGREEMENT: where the chain and the
// narrative verdict differ, and which link the narrative was silent about.
// Enforcing before that record exists would repeat this repo's most expensive
// habit — a gate switched on against an unmeasured population, discovered to
// be wrong only after it had force-FAILed real work.

import "strings"

// ShadowRecordFile is the per-cycle comparison, written into the phase
// workspace beside the artifacts it is about.
const ShadowRecordFile = "audit-chain-shadow.json"

// ShadowRecord is one cycle's chain-versus-narrative comparison.
//
// Every field exists to answer a question an operator will actually ask when
// deciding whether to promote the stage: did they agree, if not which way, on
// what reasoning, and was the judge even in a position to reason.
type ShadowRecord struct {
	Cycle int    `json:"cycle"`
	Phase string `json:"phase"`
	// ChainPresent distinguishes "the auditor produced no chain" from "the
	// chain was empty" — during rollout the first is expected and the second
	// is a defect, and conflating them would poison the very measurement the
	// stage exists to take.
	ChainPresent bool   `json:"chain_present"`
	Absence      string `json:"absence,omitempty"`

	NarrativeVerdict string `json:"narrative_verdict"`
	// ShippedVerdict and OverrodeBy are what the cycle actually carried after
	// the deterministic gates ran. Without them "the chain agreed with a PASS a
	// gate then force-FAILed" is indistinguishable from "the chain agreed with
	// a PASS that shipped" — the question a promotion decision turns on.
	ShippedVerdict string   `json:"shipped_verdict,omitempty"`
	OverrodeBy     []string `json:"overrode_by,omitempty"`

	// ChainVerdict is what the auditor's REASONING entails, on its own terms.
	// Agreement is measured against this and nothing else: it is the datum the
	// soak exists to collect.
	ChainVerdict string `json:"chain_verdict"`
	Agrees       bool   `json:"agrees"`
	// EvidenceAdjustedVerdict applies the entitlement downgrade. Kept SEPARATE
	// because folding it into Agrees pinned the headline datum to the state of
	// an artifact lookup table: a legitimately absent artifact (intent.md on a
	// cycle with no intent phase) made every clean PASS record as a
	// disagreement, and a soak whose disagreement column is a constant measures
	// nothing (review BLOCK).
	EvidenceAdjustedVerdict string   `json:"evidence_adjusted_verdict,omitempty"`
	Rationale               string   `json:"rationale"`
	Diagnoses               []string `json:"diagnoses,omitempty"`
	// MissingEvidence is what the judging phase was entitled to and not given.
	// Recorded even when the chain agrees: a chain that agreed while blind is
	// not evidence that the chain works.
	MissingEvidence []string `json:"missing_evidence,omitempty"`
	// ChainErrors are the chain's own malformations (uncited links, duplicates,
	// a chain narrated from a single artifact).
	ChainErrors []string `json:"chain_errors,omitempty"`
}

// Shadow builds the comparison for one judging phase.
//
// content is the phase's report, narrative is the verdict it declared, and
// given is the evidence the dispatch actually supplied. Total by construction:
// every input shape produces a record, because a shadow stage that can fail to
// record is a shadow stage that measures a biased sample.
func Shadow(cycle int, phase, content, narrative string, given []string) ShadowRecord {
	rec := ShadowRecord{
		Cycle:            cycle,
		Phase:            phase,
		NarrativeVerdict: narrative,
		MissingEvidence:  MissingEvidence(phase, given),
	}
	c, err := ParseChainBlock(content)
	if err != nil {
		rec.Absence = err.Error()
		rec.ChainVerdict = "absent"
		rec.Rationale = "no chain to conclude from; recorded as absent rather than inferred"
		return rec
	}
	rec.ChainPresent = true
	for _, e := range Validate(c) {
		rec.ChainErrors = append(rec.ChainErrors, e.Error())
	}
	pure := Conclude(c)
	rec.ChainVerdict = string(pure.Verdict)
	rec.Rationale = pure.Rationale
	rec.Diagnoses = pure.Diagnoses
	rec.Agrees = strings.EqualFold(strings.TrimSpace(narrative), string(pure.Verdict))
	if adj := ConcludeWithEvidence(c, phase, given); adj.Verdict != pure.Verdict {
		rec.EvidenceAdjustedVerdict = string(adj.Verdict)
	}
	return rec
}
