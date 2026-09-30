package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

// maxReasonSummaryLen bounds the short human summary attached to a verdict. Long
// enough for a one-line "why" (an audit defect list, an EGPS red_count), short
// enough to keep the ledger entry and the advisor's recall context cheap.
const maxReasonSummaryLen = 200

// VerdictReason is an immutable value object pairing a phase's bare-string
// Status (one of the existing Verdict* constants) with a short human Summary
// and the root-cause Taxonomy.
type VerdictReason struct {
	Status   string
	Summary  string
	Taxonomy Taxonomy
}

// IsPass reports whether the status is PASS.
func (v VerdictReason) IsPass() bool { return v.Status == VerdictPASS }

// Taxonomy is the {source, failure-mode, consequence} root-cause triple,
// deliberately parallel to ShipError.{Stage,Code,Class} so the codebase
// converges on one error vocabulary rather than two.
type Taxonomy struct {
	Source      string                        // origin role/phase: "audit","build","tdd","scout","infra",…
	FailureMode string                        // what went wrong: "egps-red","verdict-unparseable","compile-fail",…
	Consequence failureadapter.Classification // == the failure-adapter classification this maps to
}

// IsZero reports whether the taxonomy is empty (the PASS case).
func (t Taxonomy) IsZero() bool { return t == Taxonomy{} }

// ReasonFromDiagnostics folds a phase classifier's output into a
// VerdictReason: the Summary is the first error-severity Diagnostic message,
// falling back to the first warning, then to a status-derived default. Pure
// and nil-safe; never panics.
func ReasonFromDiagnostics(status string, diags []Diagnostic, tax Taxonomy) VerdictReason {
	summary := ""
	if msgs := cyclestate.ErrorMessages(diags); len(msgs) > 0 {
		summary = msgs[0]
	}
	if summary == "" {
		summary = firstDiagMessage(diags, cyclestate.SeverityWarning)
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
