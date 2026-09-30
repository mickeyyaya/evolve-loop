# Comment history: `acs/cycle563`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle563/predicates_test.go:3` — above `package cycle563`

```text
// Package cycle563 materialises the cycle-563 acceptance criteria for the
// scout-committed task `fix-memo-phase-dispatch` (see
// .evolve/runs/cycle-563/.evolve/evals/fix-memo-phase-dispatch.md).
//
// Root cause (fault-localization-report.md, confidence 0.97): the router
// legitimately plans "memo" after "ship" (cycle-561 routing-decision-12.json),
// but the runner-registration loop in wireOrchestratorDeps
// (go/cmd/evolve/cmd_cycle.go:406) validates the real .evolve/phases/memo
// overlay with the non-catalog-aware phasespec.ValidateUserSpec instead of
// ValidateUserSpecWithCatalog (the variant ApplyUserRouting already uses three
// lines above for the routing decision itself), so the single-word "memo"
// name is rejected and NO PhaseRunner is ever registered for it. The
// dispatcher then silently WARNs and advances past memo without ever running
// it (cyclerun_dispatch.go's missing-runner escape hatch) — explaining why
// completed_phases stops at "ship" across every PASS cycle sampled
// (555/557/558/559/561).
//
// Criterion 2 ("a real cycle produces memo artifacts end-to-end") is
// dispositioned manual+checklist in test-report.md, not a predicate here: it
// requires a live LLM-CLI subprocess dispatch (the real memo specrunner
// through the real bridge), which is neither deterministic nor safe to shell
// out to from an audit-gating unit predicate.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549/553/555/557/561
// precedent) for criteria 1+3 — each shells `go test -run '^Name$' <pkg>` over
// the real RED unit tests authored this cycle. Criterion 4 shells the exact
// `evolve doctor` invocation the eval file specifies.
```
