# Comment history: `acs/cycle1637`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1637/predicates_test.go:3` — above `package cycle1637`

```text
// Package cycle1637 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane (ADR-0076 continuation of cycle
// 1633's salvage snapshot 26680b3e):
//
//   - triage-unified-solution-synthesis → a validated, evidence-cited UNIFIED
//     commitment at the inboxbatch/triage seam that fails OPEN to independent
//     top_n selection, keeps per-member acceptance separate, closes members
//     TRANSACTIONALLY with landing, and is projected ONLY when valid.
//
// The second lane-scoped id, overlay-family-name-transport-ambiguity, was
// dropped by triage as already shipped (skip_shipped 797b8518 — an ancestor
// of this tree). R9.3 forbids predicates for non-top_n work, so nothing below
// binds to it.
//
// Provenance. Cycle 1633 built the contract to 17/17 predicates GREEN
// (go/acs/cycle1633 — still in this tree and GREEN at RED time) and FAILed
// only on the explanation-documentation citation gate. This package does NOT
// re-pin that contract line by line; 006 re-runs the whole cycle1633 package
// as ONE named-package `go test` so the salvaged behavior stays enforced in
// this cycle's audit without duplicating 750 lines. What this package ADDS are
// the two defects the salvage still carries — both found by driving the
// PRODUCTION callers, not by re-reading the code:
//
//  1. HETEROGENEITY HOLE (bug-reproduction phase, this cycle):
//     inboxbatch.UnifiedCommitment.Validate only adds NON-EMPTY campaigns to
//     its comparison set, so {unscoped, campaign:"X"} passes as homogeneous.
//     The mechanical campaignRule (inboxbatch/rules.go) never binds an
//     unscoped item to a campaign item — the operator's explicit partition —
//     so a synthesis claim must not either. 001 pins the typed seam, 002 the
//     REAL triage runner (fail-open, loud, independent top_n preserved).
//  2. CLAIM-STATE HOLE (production reachability): the triage persona's Step
//     0a.4 (`evolve inbox-mover claim "$id" "$CYCLE"`, agents/evolve-triage.md)
//     moves every selected item from .evolve/inbox/ to processing/cycle-N/
//     DURING the phase, before hooks.Classify runs processUnifiedCommitment.
//     That validator loads inboxbatch.LoadDir(<root>/.evolve/inbox) — the
//     ROOT only (LoadDir skips subdirs) — so every legitimate member is
//     "not a known inbox item" and the feature can never project in a live
//     cycle (this cycle's own record sits in processing/cycle-1637/). 003
//     drives the runner with members in the persona's post-claim state and
//     requires projection; its negatives forbid the naive "glob every
//     lifecycle dir" fix (processed/ = already landed; processing/cycle-<other>
//     = another lane's claim — unifying either double-closes an item).
//
// The remaining predicates are reachability proofs the 1633 set never pinned:
// 004 closes the transactional-closure half of the operator directive at the
// ONE lifecycle seam ship uses (inboxmover.CommittedIDs over the runner-
// emitted decision → ApplyCycleOutcome PASS promotes every member, FAIL
// promotes none); 005 is the anti-gaming half — router.Digest routes ONLY on
// the projection the triage phase computed, never on an agent-forged one.
//
// Every predicate exercises the system under test (a direct call on the typed
// seam, the REAL triage runner via triage.New + a fake core.Bridge,
// router.Digest, inboxmover.ApplyCycleOutcome, or a one-package go test), never
// a source grep (the cycle-85 ban). Adversarial axes (skills/adversarial-
// testing §6): NEGATIVE — 001, 002, the three rejection subtests of 003, the
// FAIL half of 004, every subtest of 005. EDGE — a single unscoped member among
// campaign members, a member at the inbox root beside claimed siblings,
// member_count forged to 99. SEMANTIC — validation (001), phase fail-open
// (002), lifecycle-state resolution (003), landing closure (004), routing
// trust boundary (005), prior-contract regression (006) are distinct
// behaviors.
//
// Flaky-shape hygiene: the ONE `go test` subprocess names a single package
// (./acs/cycle1633/), no wall-clock bounds, no literal PIDs, every git call is
// -C rooted, no load generators.
```

### `go/acs/cycle1637/predicates_test.go:309` — above `func TestC1637_001_UnifiedCommitmentRejectsMixedCampaignMembership(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 001–002: the heterogeneity hole — unscoped + campaign members are NOT one
// initiative (bug-reproduction phase, cycle 1637)
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1637/predicates_test.go:364` — above `t.Run("unscoped member beside a campaign member", func(t *testing.T) {`

```text
// Reachability: the same hole through the PRODUCTION validator
// (hooks.Classify → processUnifiedCommitment via triage.New(...).Run) — a
// heterogeneous backlog must yield independent commitments, loudly, with the
// independent top_n untouched. Two heterogeneity axes, both through the
// runner: the campaign partition (the hole) and the deliverable kind
// (ADR-0099 — regression guard for the salvaged half).
```

### `go/acs/cycle1637/predicates_test.go:617` — above `func TestC1637_006_SalvagedCycle1633ContractStaysGreen(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 006: the salvaged contract stays enforced — cycle 1633's predicate package
// (typed contract, fail-open runner, registry pins, campaign route) runs GREEN
// as ONE named package in this cycle's audit
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1637/predicates_test.go:642` — above `func TestC1637_007_CycleACSPackageIsGitTracked(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// 007: ship-tree tracking of this package (cycle-93 / cycle-1623 M1)
// ---------------------------------------------------------------------------
```
