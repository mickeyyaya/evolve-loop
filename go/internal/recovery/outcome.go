// Package recovery owns the Phase Recovery Pipeline (ADR-0044): given a
// phase dispatch result, produce a reconciled outcome or a typed terminal
// failure — recovery as a single-owner concern instead of point-fixes
// smeared across bridge, runner, orchestrator, and observer.
//
// cycle-262 (2026-06-09) is the motivating incident: a build whose CLI
// fallback succeeded (codex exit 81 → claude exit 0, valid PASS report,
// goal achieved) was recorded as if it never ran, because the post-phase
// tree-diff guard aborted the cycle on a real worktree leak and — like
// every abort path between dispatch return and the orchestrator's
// happy-path recording site — returned without recording the outcome.
// Reality and record diverged; the salvage had to be reconstructed by hand.
//
// Slice 1 (C1) ships the single-source outcome envelope: PhaseOutcome is
// the record EVERY terminal disposition of a phase dispatch must produce
// exactly once — happy advance and abort alike. Later slices layer
// terminal-state classification (C2), the recovery chain of responsibility
// (C3), the observer stall policy (C4), and the LLM escalation tail on top
// of this type.
//
// Leaf constraints (mirrors internal/router): recovery must stay importable
// by both core and phases/runner, so it never imports either (or bridge).
// Phase identifiers cross as plain strings; verdict strings are produced by
// the caller — core owns the canonical verdict vocabulary and the
// never-invent-PASS reconciliation rule (core.phaseOutcomeFrom), so this
// package carries no verdict constants to drift.
package recovery

// PhaseOutcome is the single-source record of one phase dispatch's terminal
// disposition (ADR-0044 C1). The orchestrator funnels every terminal path —
// success, exhausted retries, review-gate reject, worktree-leak recovery
// failure, tree-diff guard abort, persistence failure — through exactly one
// recording of this envelope, so the cycle record (PhasesRun,
// phase-timing.json, <phase>-usage.json) always reflects what actually ran,
// even when the cycle aborts.
type PhaseOutcome struct {
	// Phase is the canonical phase name ("build", "scout").
	Phase string
	// Verdict is the canonical verdict recorded for the dispatch: the
	// agent's own verdict when a canonical one exists, else a synthesized
	// FAIL. The caller guarantees canonicality; this package never
	// upgrades a verdict (a synthesized PASS is structurally impossible).
	Verdict string
	// CostUSD, DurationMS, and BootMS are the dispatch's resource usage as
	// reported by the phase response — recorded even on aborts, so burned
	// tokens are always accounted (cycle-262 lost the build's entire spend).
	CostUSD    float64
	DurationMS int64
	BootMS     int64
	// StartedAt and EndedAt are the per-phase wall-clock anchors (RFC3339):
	// the durable latency evidence. StartedAt is the dispatch start (the
	// orchestrator's PhaseStartedAt); EndedAt is stamped by recordPhaseOutcome
	// — the single C1 chokepoint — when the dispatch's outcome is finalized.
	// DurationMS above is the runner's self-reported compute; the StartedAt→
	// EndedAt span is the orchestrator-observed wall-clock and the two may
	// differ by the surrounding review/guard work (that delta is itself signal).
	StartedAt string
	EndedAt   string
	// Archetype is the phase's composition class (plan/build/evaluate/control)
	// — stamped at the chokepoint from the existing phasespec taxonomy so the
	// latency roll-up can bucket cycle time (build=productive, evaluate=checking,
	// plan=discovery, control=recovery/orchestration) without a duplicate list.
	Archetype string
	// AttemptCount is the dispatch-loop attempt count that produced this
	// outcome (matches the attempt_count key in phase-timing.json and the
	// usage sidecar, so the C1 data flow reads uniformly end to end).
	AttemptCount int
	// AbortReason is non-empty when the cycle aborted AFTER this outcome
	// was produced (review reject, leak-recovery failure, tree-diff guard,
	// ledger/state persistence failure). Verdict above remains the agent's
	// own — an abort is a cycle-level disposition, never a verdict rewrite.
	AbortReason string
}
