# internal/verdictcache

> Related: [ADR-0048](../adr/0048-work-conservation-fast-reland-resilient-ship.md) (work conservation, slices B and C1). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/verdictcache` is the content-addressed audit-reuse store of ADR-0048 slice B. It binds an audit PASS or WARN verdict to the content that the audit judged. The key is the worktree tree SHA (`git write-tree` of the staged changes), the same content identity that ship verifies in the pre-commit binding (slice C1).

## Design

- **Advisory and self-invalidating.** A lookup miss costs a full tdd, build and audit run. A lost, stale or corrupt cache costs time and never correctness. This is the degradation contract of `clihealth`. Persistence, the atomic write and the degrade-to-empty on a corrupt file mirror that package. `Load` degrades to an empty map with a stderr warning, because the cache must never break a cycle.
- **The key is the tree SHA alone.** The ADR keys on `(tree_sha, inputs_digest)`. But no code computes `inputs_digest`. It is only a parse-only field in the phasecoherence Markdown that agents emit.

  The shadow stage keys on the tree SHA, which `recordAuditBinding` already computes and persists in the ledger. The `inputs_digest` value (to also pin the eval set and the goal identity) is a refinement for the enforce stage. There a match skips phases and needs the stronger key.
- **One eligibility predicate.** `ProbeEligible` is the single source of the fresh-base collision guard. The pre-loop shadow probe (`Lookup`) and the audit-binding projection (`Put`) share it. So the two sites cannot drift, and an enforce-stage lookup has one predicate to reuse.
- **One verdict vocabulary.** `Reusable` is the single definition of the carry-forward vocabulary. Before cycle 1571 it existed three times. It was documented on `Entry.Verdict`, the store did not enforce it, and the binding call site wrote `verdict == VerdictFAIL` by hand.

  The RUNG 0 composition snapshot needs the same rule. It consulted no verdict at all, and it can carry a rejected audit forward. The predicate uses the canonical constants from `cyclestate`, a zero-dependency leaf that does not import this package.

  An import of `core` is not safe.

- **A bound on the entries.** `Put` keeps the newest `Keep(k)` entries by `cached_at` and drops the rest, so the file and its full read on each lookup stay small. A tie keeps the lower tree SHA. A store with no `Keep` call keeps the `gc.verdict_cache_keep` compiled default of 50. The orchestrator passes the project's `gc.verdict_cache_keep` (`policy.GCFor`). An evicted tree is a miss, which costs a fresh audit and nothing else.

## Invariants

- **No content identity, no cache.** An empty tree SHA never hits in `Lookup`. `Put` on it is a no-op, not an error, so a best-effort caller need not branch. Pinned by `TestPut_EmptyTreeSHA_NoOp` and `TestPut_EmptyTreeSHAStaysNoOp`.
- **A fresh base is not eligible.** A candidate equal to a resolved base tree is a fresh worktree. Every sibling lane cut from the same base carries that identity, so a verdict stored or matched under it collides across unrelated cycles. An unresolvable base (the empty string) leaves the candidate eligible. A missing base identity is not evidence of freshness, and the behaviour from before the guard stays frozen on purpose. Pinned by `TestProbeEligible_FreshBaseGuard`.
- **Only PASS and WARN are reusable.** WARN ships by the fluent-audit default. FAIL is a rejection and SKIPPED means that no audit ran, so neither can be carried forward. An out-of-vocabulary string is never trusted, and the vocabulary is upper case, so a case slip is not a silent PASS. Pinned by `reusable_test.go`.
- **The write side fails closed.** `Put` refuses a verdict that is not reusable. The store is shared and its entries outlive the cycle that wrote them. An enforce-stage lookup skips real work on the strength of a rejected tree. The asymmetry matches the binding call site. Pinned by `TestPut_RejectsNonReusableVerdict`.
- **A put survives a new Store** over the same root through an atomic write (`TestPut_Persists`). A put over a corrupt file starts fresh (`TestLoad_CorruptFile_DegradesToEmpty`).

## Findings

- **Cycle 1571 (H2)**: the cacheable vocabulary was documented and enforced nowhere. `Put` accepted any string. The `Reusable` predicate gave it one definition.
