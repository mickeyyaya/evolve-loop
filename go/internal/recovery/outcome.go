// Package recovery owns the Phase Recovery Pipeline: the phase outcome record,
// fatal-pane classification, the recovery chain, and signature promotion.
// See ADR-0044.
package recovery

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"

type PhaseOutcome struct {
	Phase         string
	Verdict       string
	CostUSD       float64
	DurationMS    int64
	BootMS        int64
	StartedAt     string
	EndedAt       string
	Archetype     string
	AttemptCount  int
	AbortReason   string
	ModelSource   string
	ResolvedModel string
	Tokens        cyclestate.TokenUsage
	Diagnostics   []cyclestate.Diagnostic
}
