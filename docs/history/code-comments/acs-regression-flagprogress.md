# Comment history: `acs/regression/flagprogress`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/flagprogress/progress_test.go:3` — above `package flagprogress`

```text
// Package flagprogress is the ACS strict-reduction guard for the flag-reduction
// campaign. During an active campaign (EVOLVE_FLAG_CAMPAIGN=1) it fails the
// per-cycle gate when a cycle did NOT delete at least one registry row — i.e.
// len(flagregistry.All) at the working tree is not strictly less than at HEAD
// (the cycle's parent commit).
//
// Why this guard exists: the sibling flagceiling guard only blocks the live
// count from RISING; nothing required it to FALL. A cycle could therefore do a
// plausible refactor (e.g. rename a reader to a "EVOLVE_"+"X" split-const) that
// makes the diff/tests/audit/adversarial-review all pass while netting ZERO row
// deletions — exactly how relaunch cycle 10 (w2-phaserecovery-ipc) shipped a PASS
// with rows 35 -> 35. Gating the METRIC (len(All)) is unforgeable in a way that
// gating the diff shape is not: the one thing a cosmetic change cannot fake is
// the row count going down.
//
// Activation: keyed off EVOLVE_FLAG_CAMPAIGN=1, set at campaign launch and
// inherited by the per-cycle `go test -tags acs` subprocess. Dormant (skip)
// everywhere else, so normal main/dev cycles are unaffected. The env literal is
// read only here in an acs _test.go, which the flagreaders guard excludes, so it
// needs no registry row of its own.
//
// Fail-CLOSED during an active campaign (ADR-0064 M3): if HEAD's registry is
// unreachable while EVOLVE_FLAG_CAMPAIGN=1, the cycle is QUARANTINED (failed),
// not skipped — a campaign cycle runs in a full clone where HEAD is always
// reachable, so an unreachable baseline is anomalous and must not silently pass.
// Outside a campaign the gate is dormant regardless of git reachability.
```

### `go/acs/regression/flagprogress/progress_test.go:78` — above `{"no change", 35, 35, false},`

```text
// cycle-10 failure mode: no rows deleted (35 -> 35) must be flagged.
```

### `go/acs/regression/flagprogress/progress_test.go:104` — above `func classifyReduction(campaignActive, parentReachable bool, current, parent int) reductionVerdict {`

```text
// classifyReduction decides the gate outcome. M3 (ADR-0064 Pillar 2): during an
// active campaign an unreachable parent is QUARANTINE, not a skip — the prior
// fail-open let a cycle bypass the strict-reduction check by making HEAD's
// registry unreadable. Outside a campaign the gate is dormant regardless.
```
