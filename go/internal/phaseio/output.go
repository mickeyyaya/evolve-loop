package phaseio

// Verdict is the typed outcome a phase emits. The vocabulary matches the EGPS
// gate (core.Verdict* strings) — held here as the leaf's canonical contract type
// so PhaseOutput carries a typed verdict rather than a bare, unvalidated string.
type Verdict string

// The four canonical phase verdicts (P5: each phase decides its own outcome).
const (
	VerdictPASS    Verdict = "PASS"
	VerdictFAIL    Verdict = "FAIL"
	VerdictWARN    Verdict = "WARN"
	VerdictSKIPPED Verdict = "SKIPPED"
)

// IsValid reports whether v is one of the four canonical verdicts. Case- and
// whitespace-sensitive — guards against silent typos.
func (v Verdict) IsValid() bool {
	switch v {
	case VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED:
		return true
	}
	return false
}

// Defect is a single structured finding in a FailureBlock.
type Defect struct {
	Severity string // canonical severity word ("LOW".."CRITICAL")
	Title    string
	Detail   string
}

// FailureBlock is the structured, machine-routable failure context a non-PASS
// phase must emit (the report-sentinel v2 failure block, generalized). Phase 3.8
// makes it mandatory for all verdict-emitting phases; here it is the typed
// carrier so the failure is data, not prose.
type FailureBlock struct {
	Class   string // failure class (e.g. "build_error", "gate_fail")
	Summary string // one-line human summary
	Defects []Defect
}

// PhaseOutput is the unified typed result a phase emits (the "message on the
// pipe", P4). It carries the semantic contract — verdict, structured failure,
// the namespaced signal bus, the deliverable's commit, the routing hint, and the
// recorded worktree tree SHA (the seam for future per-phase-worktree isolation).
// Raw telemetry (tokens/cost/duration) intentionally stays on the wire
// core.PhaseResponse: re-typing it here would be duplication with no contract
// value. The zero value is a valid empty output.
type PhaseOutput struct {
	Phase           string
	Verdict         Verdict
	Failure         *FailureBlock  // nil ⟺ no structured failure (must be set on non-PASS at enforce, Phase 3.8)
	Signals         map[string]any // namespaced <phase>.<key> signal bus this phase emits
	NextPhase       string         // routing hint (advisory; the kernel disposes)
	CommitSHA       string         // the commit anchoring this phase's deliverable (ADR-0027), empty if none
	WorktreeTreeSHA string         // recorded tree SHA of the worktree at phase end (isolation seam)
	Reconciled      bool           // bridge reported process failure but the on-disk deliverable was trusted
}

// IsPass reports whether the phase verdict is PASS.
func (o PhaseOutput) IsPass() bool { return o.Verdict == VerdictPASS }

// HasFailure reports whether a structured failure block is present.
func (o PhaseOutput) HasFailure() bool { return o.Failure != nil }
