# Comment history: `acs/cycle769`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle769/predicates_test.go:3` — above `package cycle769`

```text
// Package cycle769 materializes the cycle-769 acceptance criteria for the sole
// committed top_n task boot-orphan-sweep-bounded-tombstone (triage-report.md
// ## top_n; fleet_scope pins this lane to exactly that id — the scout's own
// proposals stay with their owning cycles, so per R9.3 no predicates bind to
// them and nothing binds to deferred/dropped work).
//
// Task source: inbox id boot-orphan-sweep-bounded-tombstone (weight 0.90):
// every loop boot re-reaps the ENTIRE run history serially with
// context.Background() — 4,388 recorded sessions across 304 registries,
// unbounded growth, and a wedged tmux hangs boot forever. Fix contract:
// (1) tombstone fully-reaped registries so sweeps stop redoing history,
// (2) partial failure leaves the registry unmarked for retry,
// (3) the preflight boot sweep is deadline-bounded (orphanGCTimeout
//
//	discipline) via an injectable killer seam.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 second sweep skips fully-reaped runs      → C769_001
//	AC2 partial failure NOT tombstoned (retried)  → C769_002
//	AC3 preflight reap deadline-bounded           → C769_003
//	AC4 go vet / -race / apicover green           → C769_004 (vet); -race is
//	    embedded in every runGoTest invocation; apicover -enforce runs in the
//	    repo-wide audit gate (ADR-0069), not re-implemented here.
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract, which EXERCISES ReapOrphans / looppreflight.Run through
// injected counting/failing/deadline-capturing killers — behavioral via
// subprocess, no source-grep predicates (cycle-85 rule). The `-v` +
// "--- PASS:" guard rejects rename/no-tests-matched silent greens. The unit
// contract embeds the adversarial axes: negative (partial failure must NOT
// be tombstoned; a sibling's failure must not re-open a successful run's
// tombstone), edge/anti-overfit (a run appearing between sweeps is still
// reaped — the skip must key on the per-run marker, not global state),
// semantic (tombstone-skip, retry-on-failure, and deadline-bounding are
// three separate behaviors in two packages).
```
