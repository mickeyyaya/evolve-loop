---
score_cap:
  - criterion: "An audit binding over a worktree with no delta against its base does not write a verdict-cache entry (a no-op audit cannot seed the fresh-base collision entry)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1495_003_NoDiffAuditDoesNotSeedCacheEntry ./acs/cycle1495"
  - criterion: "An audit over a real delta still projects into the cache under exactly the worktree tree identity the audit binding recorded (producer key == consumer key)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1495_004_ChangedAuditStillRecordsBoundTreeIdentity ./acs/cycle1495"
---

# Eval: Prevent no-op audits from seeding fresh-base verdict-cache entries

> Pins the PRODUCER half of the cycle-1495 fresh-base collision.
> `recordAuditBinding` projects the audit verdict into the ADR-0048 Slice B
> verdict cache keyed by the SAME worktree tree SHA the ledger binding records.
> Without the no-delta rule at the producer, an audit that ran over an untouched
> worktree writes an entry under the shared base-tree identity — manufacturing
> exactly the entry a later clean sibling lane collides with, so suppressing the
> consumer alone would only hide the symptom. Producer and consumer must read one
> predicate (`verdictcache.ProbeEligible`), and the changed-delta control must
> keep recording, so a blanket "stop writing to the cache" implementation fails.
> Source incident: cycle 1495 (`docs/incidents/2026-08-15-false-walls-and-repick-class.md:58`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-op-audit-does-not-seed | No-diff audit writes no cache entry (audit binding proven to have run first, so the check is non-vacuous) | 7/10 | `go test -tags acs -run TestC1495_003_...` |
| changed-audit-control | A real delta is still recorded, under the audit-bound tree identity | 6/10 | `go test -tags acs -run TestC1495_004_...` |
