package policy

const defaultIncidentEvidenceBudgetBytes = 256 * 1024

type IncidentPolicy struct {
	Evidence *IncidentEvidencePolicy `json:"evidence,omitempty"`
}

type IncidentEvidencePolicy struct {
	BudgetBytes int `json:"budget_bytes,omitempty"`
}

func (p Policy) IncidentEvidenceBudgetBytes() int {
	if p.Incident == nil || p.Incident.Evidence == nil || p.Incident.Evidence.BudgetBytes <= 0 {
		return defaultIncidentEvidenceBudgetBytes
	}
	return p.Incident.Evidence.BudgetBytes
}
