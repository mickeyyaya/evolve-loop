# Comment history: `acs/cycle1189`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1189/predicates_test.go:3` — above `package cycle1189`

```text
// Package cycle1189 materialises the cycle-1189 acceptance criteria for the
// three fleet-scoped tasks triage committed to this lane:
//
//   - loop-base-divergence-boot-halt   → boot HALTs loud when the local base has
//     diverged from / fallen behind origin/<base>, naming `evolve sync-main`
//   - bridgewatch-follow-event-sync-fix → the macOS-flaky follow test waits on an
//     event with a >=10s deadline instead of a bare 10ms sleep in a 200ms window
//   - ledger-verify-seal-anchor-fix     → Verify walks from the last `reset-seal-*`
//     operator entry forward; a pre-seal break is informational, a post-seal
//     break is still BROKEN
//
// (`codegraph-blast-radius-context-for-scout-audit-review` is `## deferred` this
// cycle, so per R9.3 it gets ZERO predicates here.)
//
// Predicate strategy — every load-bearing assertion EXERCISES the system under
// test, never greps production source for a magic string (the cycle-85
// degenerate-predicate ban):
//
//   - 001/002 build a REAL git fixture (bare origin + clone) and call
//     looppreflight.Run through its exported seams: behind-origin must HALT with
//     the reconcile instruction (001), in-sync must NOT halt (002, the
//     anti-blanket-halt negative). Both are name-agnostic — they assert on
//     Run's Result, so Builder is free to name the new check/seam anything.
//   - 003 runs the real flaky test as a subprocess under `-race -count=25`; a
//     wait that is still time-window-bound stays flaky and reds here. 004 is the
//     structural companion (deadline >=10s, no bare unconditional sleep).
//   - 005/006 drive the REAL ledger: append a chain, corrupt a line, write a
//     `reset-seal-*` operator entry, and assert Verify's verdict. 006 is the
//     crux anti-no-op: a break AFTER the seal must still return
//     core.ErrLedgerChainBroken, so "make Verify always return nil" fails.
```

### `go/acs/cycle1189/predicates_test.go:95` — above `func baseFixture(t *testing.T, behind bool) string {`

```text
// baseFixture builds a bare origin plus a working clone. When behind is true a
// SECOND clone pushes an extra commit to origin, so the returned working clone's
// local main is strictly behind origin/main WITHOUT having fetched it — exactly
// the stale-base topology that produced the cycle-969 GIT_PUSH_REJECTED. Returns
// the working clone path (the ProjectRoot handed to Run).
```
