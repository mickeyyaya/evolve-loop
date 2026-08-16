---
score_cap:
  - criterion: "verdictcache exposes a deterministic ProbeEligible(base, candidate) predicate that rejects an unchanged fresh base and an empty candidate"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestProbeEligible ./internal/verdictcache"
  - criterion: "The ADR-0048 pre-loop shadow probe derives eligibility from the shared predicate on the real RunCycle path"
    max_if_missing: 8
    evidence: "cd go && go test -tags integration -count=1 -run TestVerdictCacheProbeEligibilityWiring ./internal/core"
  - criterion: "The fresh-base collision behaviour is unchanged: same-base sibling worktrees never match a cached verdict, changed trees still do"
    max_if_missing: 8
    evidence: "cd go && go test -tags integration -count=1 -run TestVerdictCacheCollisionRegression ./internal/core"
---

# Eval: Verdict-cache fresh-base collision guard is single-sourced

> Pins the fresh-base collision guard for the ADR-0048 Slice B verdict cache.
> A fresh cycle worktree's content SHA equals its base tree SHA, so a pre-loop
> `Lookup` on that identity matched verdicts audited by sibling lanes at the same
> base and logged "would skip tdd/build/audit" — four contaminated shadow matches
> recorded in `.evolve/inbox/2026-08-14T13-10-00Z-verdict-cache-fresh-base-collision.json`.
> The behavioural guard landed as an inline comparison duplicated at two call
> sites (`orchestrator.go` probe, `phase_bindings.go` audit-binding Put), leaving
> nothing for a future enforce-stage lookup to reuse and two copies free to
> drift. This eval keeps the guard permanently expressed as ONE exported
> predicate that both production call sites reach, and keeps the observable
> behaviour frozen. Source incident: cycle 1488 (inbox item
> `verdict-cache-fresh-base-collision`, batch 2, weight 0.88).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shared-predicate | `ProbeEligible` exists and rejects fresh base / empty candidate | 7/10 | `go test -run TestProbeEligible ./internal/verdictcache` |
| probe-wiring | Shadow probe's decision agrees with the predicate on the real RunCycle path | 8/10 | `go test -tags integration -run TestVerdictCacheProbeEligibilityWiring ./internal/core` |
| behaviour-frozen | Same-base siblings never match; changed trees still do | 8/10 | `go test -tags integration -run TestVerdictCacheCollisionRegression ./internal/core` |
