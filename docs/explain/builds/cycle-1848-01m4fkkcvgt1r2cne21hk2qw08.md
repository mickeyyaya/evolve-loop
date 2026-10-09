# Build Explanation — Cycle 1848

## Build Binding
- Cycle: 1848
- Base SHA: 7d49ec1dcc8264ae085fb4a8cc4ae5cbc0b0f136

## Summary
The audit skills-drift gate now fails a lane when the worktree `evolve skills check` exits non-zero with a drift report, even if that report lands at the deadline. `worktreeSkillsDrift` derives its run context from a parent context, so cancelling the parent stops the subprocess and is reported as `context.Canceled`, not as a deadline. A missing `go` binary stays a named, NOT-graded skip WARN.

## Rationale
Before this change, `worktreeSkillsDrift` checked `ctx.Err()` before it read the report. A drift report that arrived at the deadline instant was downgraded to a skip WARN, which masked confirmed drift. Its context came from `context.Background()` and could not be cancelled from outside. The function now takes a parent context. The production caller still passes `context.Background()`. The phase hook `Classify` takes no context, and making the real phase Run context reachable needs edits to `go/internal/core/phase.go` and `go/internal/phases/runner/`. Those are protected control-plane paths (ADR-0064), so a cycle may not edit them, and that wiring is left to console work.

## Changed Areas
- `go/internal/phases/audit/skillsdrift.go` — `worktreeSkillsDrift(parent, root)` bounds the run by `parent` plus the gate limit, reads drift offenders before it classifies a context error, and reports any context error (deadline or cancellation) as NOT graded with the cause wrapped. `skillsDriftCheckDefault` passes `context.Background()`.
- `go/internal/phases/audit/skillsdrift_residuals_test.go` — the TDD phase's tests for the deadline, cancellation and missing-go residuals.
- `go/internal/phases/audit/skillsdrift_test.go` — call sites updated to the new `worktreeSkillsDrift(ctx, root)` signature.
- `go/internal/phases/audit/audit_skillsdrift_worktree_integration_test.go` — call site updated to the new signature.
- `go/acs/cycle1848/predicates_test.go` — the cycle 1848 acceptance predicates written by the TDD phase.
- `.evolve/evals/skills-drift-gate-residuals-822.md` — the task eval and its score caps.

## Design Decisions
A drift report outranks a context error: the report is direct evidence of what the gate exists to surface, and a killed run with no report still cannot grade. Any context error, not only `DeadlineExceeded`, is NOT graded, so a cancellation wraps `context.Canceled` and is not mislabelled as a timeout. The parent context is a function parameter, so a later console change can pass the phase Run context without touching this gate again.

## Verification
`go test -count=1 ./internal/phases/audit` passes, and `evolve acs suite --cycle 1848` reports red=0. The existing skills-drift outcome tests stay green.

## Compatibility
The exported `Config.CheckSkillsDrift` hook signature is unchanged, and so are `core.PhaseRequest` and the runner. In production the run is bounded by the gate limit exactly as before.

## Limitations
In production the phase Run context does not reach the gate yet: `skillsDriftCheckDefault` passes `context.Background()`. That plumbing needs protected paths (`go/internal/core/phase.go`, `go/internal/phases/runner/`) and is console work.
