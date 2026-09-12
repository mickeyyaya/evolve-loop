# Build Explanation — Cycle 1639

## Build Binding
- Cycle: 1639
- Base SHA: 4c58eb6d1da8f5f7a26d677c0263e39b9e7842d8

## Summary
An explicit empty triage commitment now classifies the cycle as planned no-work even when triage returns FAIL, preventing dispatch of the implementation spine.

## Rationale
The committed `top_n` is the authoritative evidence of eligible work. Giving it precedence over the carrier verdict preserves genuine failures for non-empty or missing commitments while correctly closing a known no-work cycle.

## Changed Areas
- `go/internal/core/triage_termination.go` — checks an explicit empty commitment before verdict-specific failure handling so the terminal selector records the no-work reason for PASS, WARN, and FAIL.
- `go/internal/core/cycle_closeout.go` — accepts the already-authorized no-work disposition independent of the triage verdict, keeping resumed-cycle closeout aligned with selection.
- `go/internal/core/empty_commitment_dispatch_test.go` — adds fresh and resumed FAIL-with-empty coverage, plus non-empty and absent-decision controls that prove normal failure behavior remains intact.

## Design Decisions
The change reuses `RoutingSignals.HasEmptyTriageCommitment` and the existing `CycleTerminationTriageNoWork` disposition rather than adding a new routing signal or configuration surface. Unknown, malformed, and non-empty commitments do not take this path.

## Verification
Targeted core tests cover fresh and resumed dispatch, final disposition, worktree cleanup, and anti-no-op controls. The established ACS composed-path tests also remain green.

## Compatibility
No public API, state schema, or routing-plan shape changes. Existing PASS and WARN empty-commitment behavior is preserved.

## Limitations
This change only treats an explicit empty commitment as planned no-work; it intentionally does not reinterpret missing or invalid triage decisions.
