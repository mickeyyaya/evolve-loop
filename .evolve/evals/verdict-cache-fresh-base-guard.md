---
score_cap:
  - criterion: "A worktree whose content tree equals the cycle's base tree is never a verdict-cache lookup key"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1492_001_FreshBaseTreeIsNeverALookupKey ./acs/cycle1492"
  - criterion: "The orchestrator's pre-loop shadow probe suppresses the lookup on a fresh base even when that key holds a cached PASS"
    max_if_missing: 8
    evidence: "cd go && go test -tags integration -count=1 -run 'TestVerdictCacheCollisionRegression/clean_cached_base_is_suppressed' ./internal/core"
---

# Eval: verdict-cache fresh-base guard (ADR-0048 Slice B)

> Pins the fresh-base exclusion at the ADR-0048 Slice B verdict-cache probe. The
> pre-loop probe runs before tdd/build/audit, so `git write-tree` on an untouched
> worktree returns the base commit's tree — an identity every sibling lane cut
> from the same main tip shares. Under enforce, a match on that key would skip
> the tdd/build/audit spine of an unbuilt cycle and carry a stranger's verdict
> forward. Source incident: `.evolve/inbox/2026-08-14T13-10-00Z-verdict-cache-fresh-base-collision.json`
> (batch 2026-08-14, cycles 1457-1460: four contaminated matches on tree
> `b1ae51b3` spanning PASS and WARN, including a lane whose inbox item was
> minutes old; and batch-20260815c wave-1, where two sibling fresh worktrees both
> matched cycle 1477 PASS). Re-derived and carried through cycles 1488 and 1492.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fresh-base-not-a-key | Base-equal tree is rejected as a cache key, proven over real `git write-tree` identities | 8/10 | `go test -tags acs -run TestC1492_001_FreshBaseTreeIsNeverALookupKey ./acs/cycle1492` |
| caller-proof | The suppression is reached from the real RunCycle pre-loop probe, not merely available | 8/10 | `go test -tags integration -run 'TestVerdictCacheCollisionRegression/clean_cached_base_is_suppressed' ./internal/core` |
