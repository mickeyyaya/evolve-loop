// Package recovery owns the Phase Recovery Pipeline: the phase outcome record,
// fatal-pane classification, the recovery chain, and signature promotion.
// See ADR-0044.
package recovery

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"

// PhaseOutcome is the record every terminal disposition of a phase dispatch produces exactly once.
type PhaseOutcome struct {
	Phase string
	// Verdict is the agent's canonical verdict, else a synthesized FAIL; an abort never rewrites it.
	Verdict string
	// CostUSD, DurationMS and BootMS are recorded even when the cycle aborts.
	CostUSD    float64
	DurationMS int64
	BootMS     int64
	// StartedAt and EndedAt (RFC3339) are the orchestrator-observed wall clock;
	// DurationMS is the runner's self-reported compute, and the gap is signal.
	StartedAt string
	EndedAt   string
	// Archetype is the phase's composition class: plan, build, evaluate or control.
	Archetype    string
	AttemptCount int
	// AbortReason is also set when a recovery path recorded a transient and
	// continued, so a non-empty value does not prove the cycle died here.
	AbortReason string
	// ModelSource names the resolution path that won: "profile", "pin" or "advisor".
	ModelSource   string
	ResolvedModel string
	// Tokens is the terminal attempt's usage, not a sum across attempts.
	Tokens cyclestate.TokenUsage
	// Diagnostics are relayed unfiltered: this record is the only durable home
	// of a non-floor phase's FAIL reason.
	Diagnostics []cyclestate.Diagnostic
}
