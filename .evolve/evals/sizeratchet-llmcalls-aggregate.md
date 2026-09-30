---
score_cap:
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists internal/llmcalls.Aggregate"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_007_LlmcallsAggregateOffenderEntryRemoved ./acs/cycle1777"
  - criterion: "internal/llmcalls.Aggregate measures at most 50 lines under sizeratchet.Walk"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1777_008_LlmcallsAggregateWithinRatchetLimit ./acs/cycle1777"
  - criterion: "the internal/llmcalls package's own test suite stays green, including Aggregate's deterministic output ordering"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/llmcalls/..."
---

# Eval: Drop the stale internal/llmcalls.Aggregate offenders.json entry

> `internal/llmcalls.Aggregate` is listed in `go/internal/sizeratchet/offenders.json`
> with allowance 53, but at TDD authoring time (cycle 1777, 2026-09-30, HEAD
> `a547b474`) the live function already measures 32 lines under
> `sizeratchet.Walk`. This is the same function cycle 1771
> (`go/acs/cycle1771/predicates_test.go`, shipped) already extracted the
> grouping/keying step from, en route to shrinking it from 53 to its current
> 32 lines; `Aggregate` already sorts its output with a total-order
> comparator (`lessPerformance`, comparing all seven grouping-key fields), so
> repeated calls on identical input are already deterministic by
> construction. Cycle 1771 deliberately left the `offenders.json` entry as
> "boundary tighten" slack (see that cycle's eval,
> `.evolve/evals/sizeratchet-shrink-usage-scanners.md`) rather than dropping
> it. This cycle's scout independently re-derived `sizeratchet-llmcalls-aggregate`
> as a lane task straight from the still-listed `offenders.json` entry,
> without visibility into cycle 1771's already-completed extraction — the
> only genuinely outstanding work is removing the stale entry. Source:
> cycle-1777 triage-report.md top_n (`sizeratchet-llmcalls-aggregate`).
>
> **Surfaced convention conflict:** see the companion note in
> `.evolve/evals/sizeratchet-verifyeval-shellwords.md`. This cycle's
> triage-report.md and scout-report.md independently state "drop
> offenders.json entry" as the acceptance bar; no checked-in policy document
> codifies cycle 1770/1771's "leave it as slack" convention as a hard rule.
> Resolution: follow this cycle's explicit acceptance text and drop the
> entry — safe either way under `sizeratchet.Check`'s own stale-entry-tolerant
> semantics (`TestCheck_OnlyGrowthOrANewOffenderFails`).

## Criteria

1. **[code]** `internal/llmcalls.Aggregate`'s `offenders.json` entry is removed.
2. **[code]** `internal/llmcalls.Aggregate` stays at or under the 50-line ratchet limit.
3. **[code]** No behavior change: `internal/llmcalls`'s existing test suite (deterministic ordering included) stays green.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| entry-dropped | offenders.json entry removed | 8/10 | `go test -tags acs -run TestC1777_007_...` |
| size-fit | Aggregate ≤ 50 lines | 6/10 | `go test -tags acs -run TestC1777_008_...` |
| suite-green | llmcalls package tests pass | 9/10 | `go test ./internal/llmcalls/...` |
