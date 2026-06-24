package core

import "github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"

// maxReasonSummaryLen bounds the short human summary attached to a verdict. Long
// enough for a one-line "why" (an audit defect list, an EGPS red_count), short
// enough to keep the ledger entry and the advisor's recall context cheap.
const maxReasonSummaryLen = 200

// VerdictReason is an immutable value object pairing a phase's bare-string Status
// (one of the existing Verdict* constants) with a short human Summary and the
// root-cause Taxonomy.
//
// It does NOT replace the wire types: PhaseResponse.Verdict and
// FailedRecord.Verdict keep their bare string for ledger / hash-chain / JSON
// stability. VerdictReason is DERIVED from a phase's Diagnostics and carries the
// extra signal the orchestrator needs to react (classify → PROCEED/RETRY/BLOCK)
// and the advisor needs for recall — turning a bare "FAIL" into "FAIL, because …".
type VerdictReason struct {
	Status   string
	Summary  string
	Taxonomy Taxonomy
}

// IsPass reports whether the status is PASS.
func (v VerdictReason) IsPass() bool { return v.Status == VerdictPASS }

// Taxonomy is the {source, failure-mode, consequence} root-cause triple (the
// AgentDoG model). It is deliberately parallel to ShipError.{Stage,Code,Class}
// so the codebase converges on ONE error vocabulary rather than two.
//
// Consequence is typed as failureadapter.Classification on purpose: it IS the
// bridge to the deterministic failure-adapter, so a mismatch is a compile error
// rather than a runtime surprise (stronger than the string-guard the plan
// hedged toward). Source/FailureMode stay free strings — they are descriptive,
// not control-flow.
type Taxonomy struct {
	Source      string                        // origin role/phase: "audit","build","tdd","scout","infra",…
	FailureMode string                        // what went wrong: "egps-red","verdict-unparseable","compile-fail",…
	Consequence failureadapter.Classification // == the failure-adapter classification this maps to
}

// IsZero reports whether the taxonomy is empty (the PASS case).
func (t Taxonomy) IsZero() bool { return t == Taxonomy{} }

// ReasonFromDiagnostics folds a phase classifier's output into a VerdictReason.
// The classifiers already emit the human "why" as an error-severity Diagnostic
// (e.g. audit's "EGPS: red_count=N"), so the Summary is the first error message,
// falling back to the first warning, then to a status-derived default. PURE and
// nil-safe; never panics.
func ReasonFromDiagnostics(status string, diags []Diagnostic, tax Taxonomy) VerdictReason {
	summary := firstDiagMessage(diags, "error")
	if summary == "" {
		summary = firstDiagMessage(diags, "warning")
	}
	if summary == "" && status != VerdictPASS {
		summary = "unspecified " + status
	}
	return VerdictReason{
		Status:   status,
		Summary:  truncateReason(summary),
		Taxonomy: tax,
	}
}

// firstDiagMessage returns the message of the first diagnostic with the given
// severity, or "" if none match.
func firstDiagMessage(diags []Diagnostic, severity string) string {
	for _, d := range diags {
		if d.Severity == severity {
			return d.Message
		}
	}
	return ""
}

// truncateReason clamps a summary to maxReasonSummaryLen runes. Rune-wise (not
// byte-wise) so a non-ASCII defect message can never be cut mid-rune into
// invalid UTF-8 in the ledger.
func truncateReason(s string) string {
	if len(s) <= maxReasonSummaryLen {
		return s // fast path: byte len ≤ cap ⇒ rune count ≤ cap
	}
	r := []rune(s)
	if len(r) <= maxReasonSummaryLen {
		return s // byte-heavy but short rune count (e.g. CJK); already within cap
	}
	return string(r[:maxReasonSummaryLen])
}
