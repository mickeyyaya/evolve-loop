---
score_cap:
  - criterion: "A changed worktree stays distinguishable from its base and remains eligible for the advisory verdict-cache lookup"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1492_002_ChangedWorktreeStaysEligible ./acs/cycle1492"
  - criterion: "The orchestrator's probe decision is derived from the shared verdictcache.ProbeEligible predicate, not a re-introduced local copy"
    max_if_missing: 7
    evidence: "cd go && go test -tags integration -count=1 -run TestVerdictCacheProbeEligibilityWiring ./internal/core"
---

# Eval: verdict-cache changed-worktree reuse must survive the guard

> The anti-gaming half of the fresh-base fix. The cheapest way to stop
> fresh-base collisions is to disable verdict-cache reuse entirely — which would
> pass a collision test while discarding the genuine fast-re-land reuse ADR-0048
> Slice B exists for (its cycles 247-248 motivation). This eval pins the opposite
> direction: a worktree carrying real post-build content must still differ from
> its base, still reach the advisory lookup, and still match a seeded entry, with
> the orchestrator's decision continuing to come from the single-sourced
> `verdictcache.ProbeEligible` predicate shared with the audit-binding Put.
> Source incident: cycle 1488 / 1492 continuation of
> `verdict-cache-fresh-base-collision`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-blanket-disable | Changed content is still an eligible cache key and still matches a seeded hit | 7/10 | `go test -tags acs -run TestC1492_002_ChangedWorktreeStaysEligible ./acs/cycle1492` |
| single-sourced-decision | Orchestrator probe decision agrees with `verdictcache.ProbeEligible` on the real RunCycle path | 7/10 | `go test -tags integration -run TestVerdictCacheProbeEligibilityWiring ./internal/core` |
