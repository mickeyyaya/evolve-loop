# Build Explanation — Cycle 1947

## Build Binding
- Cycle: 1947
- Base SHA: 6705ed4f6b0c93f84af147348c357d30f6be3c8a

## Summary
The convergence policy gains a seeded property test through `Decide`, a distinct cause for the landing-guard Stop, and one worded once-only raise rule in the spec, the ADR and the package page.

## Rationale
The landing guard (`keptRoundHasOpenCritical`) is a backstop for a keep-best bug. Its Stop borrowed the next rung's cause, so an operator could not tell it from a scope-change Stop. Naming the cause makes a trip visible, and the property test asserts a trip never happens on a correct policy. Rejected: a new field on `Decision`, because it changes the public schema for a diagnostic; the cause travels in a reason line instead.

## Changed Areas
- `go/internal/convergence/decide.go` — the guard now returns `landingGuardStop()` instead of an anonymous `stop("")`.
- `go/internal/convergence/reasons.go` — adds `landingGuardStop`, the `CONVERGENCE_LANDING_GUARD` reason and the `cause=landing-guard` rung reason.
- `go/internal/convergence/property_test.go` — the seeded sweep of 3000 inputs per mode, with a residue mode that makes keep-best prefer a round with an open CRITICAL under a mass-first mutant.
- `go/internal/convergence/landing_stop_cause_test.go` — TDD-owned tests for the guard cause.
- `go/internal/convergence/feedback_once_test.go` — TDD-owned pin of the once-only raise at `max_fix_rounds` 4.
- `go/acs/cycle1947/predicates_test.go` — TDD-owned predicates, including the two mutation kills.
- `.evolve/evals/convergence-v1-review-followups.md` — the eval for this item.
- `.evolve/evals/convergence-v1-never-land-critical-property.md` — the eval for the property.
- `docs/architecture/convergence-policy.md` — states the once-only raise rule at rung 1.
- `docs/architecture/adr/0126-every-iterative-loop-converges-or-escalates.md` — states the same rule in the round list.
- `docs/architecture/packages/internal-convergence.md` — states the rule, its pin and the guard cause.

## Design Decisions
The guard cause reuses the reason channel, so `Decision` keeps its schema. The property asserts two things: no Land, Split or AcceptWithLimits holds an open CRITICAL, and the guard never fires. The first alone cannot see a keep-best defect, because the guard converts it into a Stop.

## Verification
`go test -count=1 ./internal/convergence` passes. The ACS predicates `TestC1947_001` to `TestC1947_006` pass, including the keep-best mass-first mutant and the `exempted` CRITICAL-clause mutant, which the property test kills.

## Compatibility
`Decision` keeps its fields. A guard Stop gains one extra reason line and a new `cause` value on its rung reason; no other Stop changes.

## Limitations
The guard is unreachable on the real policy, so the Stop cause is proven only through the `keptRoundHasOpenCritical` test seam.
