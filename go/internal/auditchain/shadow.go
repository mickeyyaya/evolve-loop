package auditchain

import "strings"

const ShadowRecordFile = "audit-chain-shadow.json"

type ShadowRecord struct {
	Cycle        int    `json:"cycle"`
	Phase        string `json:"phase"`
	ChainPresent bool   `json:"chain_present"`
	Absence      string `json:"absence,omitempty"`

	NarrativeVerdict string   `json:"narrative_verdict"`
	ShippedVerdict   string   `json:"shipped_verdict,omitempty"`
	OverrodeBy       []string `json:"overrode_by,omitempty"`

	ChainVerdict            string   `json:"chain_verdict"`
	Agrees                  bool     `json:"agrees"`
	EvidenceAdjustedVerdict string   `json:"evidence_adjusted_verdict,omitempty"`
	Rationale               string   `json:"rationale"`
	Diagnoses               []string `json:"diagnoses,omitempty"`
	MissingEvidence         []string `json:"missing_evidence,omitempty"`
	ChainErrors             []string `json:"chain_errors,omitempty"`
}

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
