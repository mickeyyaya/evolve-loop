# Comment history: `acs/cycle536`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle536/predicates_test.go:3` — above `package cycle536`

```text
// Package cycle536 materialises the cycle-536 acceptance criteria for the THREE
// triage-committed (`## top_n`) tasks.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly THREE tasks to this cycle:
//	  1. carryover-legacy-expiry-backfill      (H) — C536_001..004
//	  2. carryover-cycles-unpicked-increment   (M) — C536_005..006
//	  3. wire-fleet-width-topn-selection       (H) — C536_007..008
//	plus one shared no-regression hygiene predicate — C536_009.
//	Every `## deferred` item (5 stamped legacy todos, the 6 token-optimization
//	inbox items, next-cycle-brief staleness) gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//   - Task 1: `state.json:carryoverTodos` grew to 76 entries; 71 predate the
//     TTL-stamping fix and carry NO `expiresAt`, so PruneExpiredCarryoverTodos
//     keeps them forever (age unknown → never delete). They re-render into every
//     advisor/router prompt and are on track to force a manual array-wipe (the
//     cycle-360 incident). A ONE-TIME backfill stamps a conservative default TTL
//     on every legacy row so the existing prune path can converge them.
//   - Task 2: `CyclesUnpicked` is a dead field — every write site hardcodes 0 and
//     nothing increments it, yet the advisor prompt renders it as a staleness
//     signal. A per-boot increment for every SURVIVING carryover todo makes the
//     field mean what its name promises.
//   - Task 3: `triagecap.SelectFleetWidthTopN` (fleet-width-aware, file-disjoint
//     top_n packing) ships fully tested but has ZERO production callers across two
//     prior processing attempts (cycles 508, 518). This cycle wires it into the
//     wave-seed path so `evolve cycle run` supplies >= fleet.count mutually
//     file-disjoint lanes instead of a raw weight-sorted top-N (which can hand the
//     fleet two candidates that collide on a shared file).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real production function against a seeded temp state.json / temp
// inbox and asserts on the returned value AND the on-disk side effect, never a
// "source file contains text X" grep. C536_009 runs `go build`/`go vet` as real
// subprocesses.
//
// The suite is RED on the current tree by COMPILE FAILURE: the three Builder
// symbols below are all undefined —
//   - failurelog.BackfillLegacyCarryoverExpiry(statePath string, defaultTTL time.Duration, now time.Time) (stamped int, err error)
//   - failurelog.IncrementCarryoverUnpicked(statePath string) (incremented int, err error)
//   - triagecap.SelectWaveSeedTopN(evolveDir string, count int) []triagecap.FleetCandidate
//
// (see test-report.md handoff for the full Builder contract).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C536_001 stamps legacy + leaves already-stamped untouched.
//   - Idempotent: C536_002 a second backfill stamps zero more (anti-double-stamp).
//   - End-to-end: C536_003 backfilled entry is actually PRUNED once past its TTL.
//   - Edge     : C536_004 / C536_006 missing & empty state are safe no-ops.
//   - Positive : C536_005 increments 0,2,5 -> 1,3,6 by EXACTLY one (anti-reset).
//   - Negative : C536_007 the disjoint pick REJECTS the raw top-weight pair that
//     shares a file — a weight-only implementation FAILS here (the anti-no-op).
//   - Edge     : C536_008 count<2 reproduces the legacy single-focus pick.
//   - Hygiene  : C536_009 touched packages build + vet clean (no regression).
```
