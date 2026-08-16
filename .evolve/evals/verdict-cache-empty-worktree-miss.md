---
score_cap:
  - criterion: "A clean staged worktree (content identical to its base tree) cannot produce an ADR-0048 Slice B shadow verdict-cache reuse match, even when the cache already holds an entry under that tree SHA"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1495_001_CleanWorktreeProducesNoShadowReuseMatch ./acs/cycle1495"
  - criterion: "A changed staged worktree still produces a content identity and keeps the normal verdict-cache lookup path available"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1495_002_ChangedWorktreeKeepsCacheEligibility ./acs/cycle1495"
  - criterion: "The fresh-base guard is a single shared predicate (verdictcache.ProbeEligible) with frozen edge semantics: no content identity and candidate==resolved base are ineligible; an unresolvable base leaves the candidate eligible"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1495_005_ProbeEligibleSharedPredicateEdges ./acs/cycle1495"
---

# Eval: Reject clean-worktree verdict-cache probes

> Pins the CONSUMER half of the fresh-base collision closed in cycle 1495. The
> ADR-0048 Slice B pre-loop shadow probe content-addresses the worktree with
> `git add -A` + `git write-tree`. A fresh lane worktree is a clean clone of its
> base, so that identity is shared by every sibling fleet lane cut from the same
> base — a cache lookup under it matches unrelated work and contaminates the
> shadow measurement the enforce stage is supposed to be sized from. The
> suppression must be expressed by the single shared predicate both call sites
> read, never as a local copy, and it must not weaken reuse for a real delta.
> Source incident: `docs/incidents/2026-08-15-false-walls-and-repick-class.md:58`
> (queued as `verdict-cache-fresh-base-collision`); ADR-0048 Slice B.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fresh-base-suppressed | Clean worktree at the base tree never reports a shadow reuse match | 7/10 | `go test -tags acs -run TestC1495_001_...` |
| changed-control | A real delta stays cache-eligible (rejects a blanket cache disable) | 6/10 | `go test -tags acs -run TestC1495_002_...` |
| shared-predicate-edges | Empty / equal-to-base / differing / unresolvable-base semantics are frozen in one predicate | 5/10 | `go test -tags acs -run TestC1495_005_...` |
