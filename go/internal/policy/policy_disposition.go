package policy

// policy_disposition.go — the .evolve/policy.json "failure_disposition" block
// (failure-disposition-router S3/S4). Stage and escalation formula are
// CONFIG-INJECTED with compiled Go defaults, never feature flags and never Go
// literals baked into the consumer: the boundary applier reads
// threshold/step/cap from here and hands them to recurrence.EscalationPolicy.
//
// Stage defaults to the already-resolved chronicle.escalation stage rather than
// re-spelling "shadow" — one source (the chronicle block) with a projection, so
// an operator who moves recurrence escalation to enforce does not have to set
// the same word twice.

// FailureDispositionPolicy is the .evolve/policy.json "failure_disposition"
// block. Every field is optional; an absent field keeps its compiled default.
type FailureDispositionPolicy struct {
	// Stage selects the boundary applier's stage: "shadow" (report only) or
	// "enforce" (mutate the inbox). Absent ⇒ chronicle.escalation's stage.
	Stage string `json:"stage,omitempty"`
	// Threshold is the recurrence count at or above which a pattern escalates.
	Threshold int `json:"threshold,omitempty"`
	// Step is the per-extra-occurrence weight increment.
	Step float64 `json:"step,omitempty"`
	// Cap is the maximum weight escalation may reach.
	Cap float64 `json:"cap,omitempty"`
}

// FailureDispositionConfig is the resolved configuration with defaults applied.
type FailureDispositionConfig struct {
	Stage     string
	Threshold int
	Step      float64
	Cap       float64
}

// Enforce reports whether the boundary applier may mutate the inbox. Any stage
// other than "enforce" (including the shadow default and an unknown word) is
// report-only — the safe direction for an unrecognized stage.
func (c FailureDispositionConfig) Enforce() bool { return c.Stage == "enforce" }

// FailureDispositionConfig resolves the failure_disposition block against the
// compiled defaults: stage = the chronicle escalation stage (itself "shadow"
// when unset), threshold 2, step 0.03, cap 0.99 — the same constants
// recurrence.DefaultEscalationPolicy carries, kept in sync by
// policy_disposition_test.go.
func (p Policy) FailureDispositionConfig() FailureDispositionConfig {
	c := FailureDispositionConfig{
		Stage:     p.ChronicleConfig().Escalation,
		Threshold: 2,
		Step:      0.03,
		Cap:       0.99,
	}
	if p.FailureDisposition == nil {
		return c
	}
	if p.FailureDisposition.Stage != "" {
		c.Stage = p.FailureDisposition.Stage
	}
	if p.FailureDisposition.Threshold != 0 {
		c.Threshold = p.FailureDisposition.Threshold
	}
	if p.FailureDisposition.Step != 0 {
		c.Step = p.FailureDisposition.Step
	}
	if p.FailureDisposition.Cap != 0 {
		c.Cap = p.FailureDisposition.Cap
	}
	return c
}
