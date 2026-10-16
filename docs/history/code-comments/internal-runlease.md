# Comment history: `internal/runlease`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d10

### `go/internal/runlease/runlease_test.go:91` — above `func TestOwnerLive_DeadPidFreshHeartbeatNotLive(t *testing.T) {`

```text
// TestOwnerLive_* — cycle-554 workspace-hygiene-s1 RED contract. Today
// liveness is freshness-only (10min lease TTL), so a crashed owner with a
// still-fresh heartbeat (the common 2-6min post-crash window) reads as "live"
// and blocks SealCycle/the loop guard, forcing `evolve cycle reset --force` at
// every batch boundary (plan docs/plans/workspace-hygiene-2026-07.md §S1).
// OwnerLive adds a pid-liveness probe on top of Fresh: dead pid + fresh
// heartbeat ⇒ not live (the fix); live pid + fresh heartbeat ⇒ still live (the
// invariant the fence exists for, unchanged).
```
