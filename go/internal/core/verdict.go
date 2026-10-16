package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

const maxReasonSummaryLen = 200

type VerdictReason struct {
	Status   string
	Summary  string
	Taxonomy Taxonomy
}

func (v VerdictReason) IsPass() bool { return v.Status == VerdictPASS }

type Taxonomy struct {
	Source      string
	FailureMode string
	Consequence failureadapter.Classification
}

func (t Taxonomy) IsZero() bool { return t == Taxonomy{} }

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

func truncateReason(s string) string {
	if len(s) <= maxReasonSummaryLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxReasonSummaryLen {
		return s
	}
	return string(r[:maxReasonSummaryLen])
}
