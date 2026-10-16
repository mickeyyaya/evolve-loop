# Comment history: `internal/verdictcache`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/verdictcache/apicover_named_test.go:5` — above `func TestProbeEligible_FreshBaseGuard(t *testing.T) {`

```text
// TestProbeEligible_FreshBaseGuard names verdictcache.ProbeEligible and pins the
// shared fresh-base guard both production call sites (the ADR-0048 shadow probe
// and the audit-binding Put) route through: only a candidate that carries a
// content identity distinct from a RESOLVED base is cache-eligible, while an
// unresolvable base leaves the candidate eligible (frozen pre-guard behaviour).
```

### `go/internal/verdictcache/reusable_test.go:8` — above `func TestReusable_Vocabulary(t *testing.T) {`

```text
// reusable_test.go — cycle-1571 H2. The cacheable-verdict vocabulary was
// documented on Entry.Verdict ("PASS or WARN") and enforced nowhere: Put
// accepted any string, and the FAIL exclusion lived as an inline `verdict ==
// VerdictFAIL` check in core/phase_bindings.go. Two consumers now need the same
// rule — the cache write side and the RUNG 0 composition snapshot, which must
// refuse to carry a REJECTED audit forward — so the rule gets one definition.
```

### `go/internal/verdictcache/reusable_test.go:35` — above `func TestPut_RejectsNonReusableVerdict(t *testing.T) {`

```text
// TestPut_RejectsNonReusableVerdict: the store is the last line of defence for
// its own vocabulary. A poisoned entry sits in a SHARED file for every future
// consumer, and the enforce-stage lookup (ADR-0048 Slice B) would skip real
// work on its strength — so the write side fails closed, matching the
// deliberately asymmetric posture already documented at the Put call site.
```

### `go/internal/verdictcache/verdictcache.go:1` — above `package verdictcache`

```text
// Package verdictcache is the ADR-0048 Slice B content-addressed audit-reuse
// store: it binds an audit PASS/WARN verdict to the CONTENT it audited, keyed by
// the worktree tree SHA (git write-tree of the staged changes) — the same
// content identity ship verifies in the pre-commit binding (ADR-0048 Slice C1).
//
// The cache is ADVISORY and SELF-INVALIDATING: a lookup miss costs a full
// tdd/build/audit run, so a lost, stale, or corrupt cache only ever costs time,
// never correctness — the same degradation contract as clihealth. Persistence,
// atomic write, and degrade-to-empty-on-corrupt mirror that package.
//
// NOTE on the ADR (ADR-0048 Slice B): the ADR keys on (tree_sha, inputs_digest).
// inputs_digest is NOT a machine-computed value anywhere in the codebase (it is
// only a parse-only field in agent-emitted phasecoherence Markdown). The shadow
// stage therefore keys on the worktree tree SHA alone — already computed and
// ledger-persisted by recordAuditBinding. inputs_digest (to also pin eval-set /
// goal identity) is a refinement deferred to the enforce stage, where actually
// SKIPPING phases on a match demands the stronger key.
```

### `go/internal/verdictcache/verdictcache.go:89` — above `func ProbeEligible(baseTreeSHA, candidateTreeSHA string) bool {`

```text
// ProbeEligible reports whether candidateTreeSHA may be used as a verdict-cache
// key given the worktree's base tree identity. It is the SINGLE source of the
// fresh-base collision guard, shared by the ADR-0048 Slice B pre-loop shadow
// probe (Lookup) and the audit-binding projection (Put), so the two sites can
// never drift apart and a future enforce-stage lookup has one predicate to
// reuse.
//
// A candidate with no content identity is never eligible — there is nothing to
// content-address, matching Lookup/Put's empty-SHA no-ops. A candidate equal to
// a RESOLVED base tree is a fresh/untouched worktree: every sibling lane cut
// from the same base carries that identity, so a verdict stored or matched
// under it collides across unrelated cycles. An unresolvable base ("") leaves
// the candidate eligible — absence of a base identity is not evidence of
// freshness, and the pre-guard behaviour there is deliberately frozen.
```

### `go/internal/verdictcache/verdictcache.go:110` — above `func Reusable(verdict string) bool {`

```text
// Reusable reports whether a verdict may let a later run skip work on its
// strength. PASS and WARN ship (WARN by the fluent-audit default); FAIL is a
// rejection and SKIPPED means no audit ran, so neither may be carried forward,
// and an out-of-vocabulary string is never trusted.
//
// This is the SINGLE definition of the cacheable/carry-forward vocabulary.
// Before cycle-1571 it existed three times over: documented on Entry.Verdict,
// enforced nowhere by the store, and hand-written as `verdict == VerdictFAIL`
// at the binding call site — while the RUNG 0 composition snapshot, which needs
// the same rule, consulted no verdict at all and would carry a REJECTED audit
// forward. One predicate, so the three sites cannot drift (§3.5).
```
