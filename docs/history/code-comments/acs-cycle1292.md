# Comment history: `acs/cycle1292`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1292/predicates_test.go:3` — above `package cycle1292`

```text
// Package cycle1292 materialises the cycle-1292 acceptance criteria for the
// single fleet-scoped lane pinned to this cycle (inbox item
// `continuation-defect-ledger`, fifth hop of the 1279 → 1282 → 1285/1287 → 1290
// → 1292 chain). It closes the two defects the immediate ancestor's disposition
// ledger carried forward OPEN (`.evolve/runs/cycle-1290/defect-dispositions.json`):
//
//   - 1290-D2 → the partial-write OVERCLAIM. writeInboxItems writes one file per
//     item and returns on the FIRST failure, so items before the failing one are
//     already on disk; preserveDiagnosis nonetheless lists every configured item
//     as "still UNQUEUED" in retrospective-unqueued.md.
//   - 1290-D1 → the UNBACKED DEFERRAL. Both governed documents assert 1287-F2 was
//     "queued as audit-eval-existence-path-convention", and no such item exists
//     in .evolve/inbox — an unbacked claim inside the very lane that exists to
//     catch unbacked claims.
//
// Predicate strategy. 001/002 drive the production entry point
// (faillearn.WriteArtifacts) from OUTSIDE the package and assert on the emitted
// artifact's content, so they survive both cheap gaming moves at once: deleting
// the in-package reproducer, and asserting `err != nil` without ever reading the
// artifact. 003 then requires the in-package reproducer to be tree-resident and
// run-and-pass together with the pre-existing 1287/1290 invariants — greening 001
// by weakening inbox_transactional_test.go or inbox_failure_degraded_test.go is
// the fix being wrong, not the contract being met. 004 drives the REAL inbox
// consumer (inboxbatch.LoadDir) rather than stat-ing a filename, because the
// defect is "the claim is unbacked", and a file the loader drops backs nothing.
// Subprocess predicates run ONE named package under an explicit -run expression
// with per-name PASS accounting, per the flaky-predicate-shape rules.
```

### `go/acs/cycle1292/predicates_test.go:137` — above `collision, err := json.MarshalIndent(faillearn.InboxItem{ID: items[1].ID, Title: "filed by another lane", Weight: 0.5, K…`

```text
// A DIFFERENT item already filed under item 2's id — the cycle-1282 DEF-4
// collision rule refuses to drop ours, which fails the write at index 1.
```

### `go/acs/cycle1292/predicates_test.go:203` — above `func TestC1292_003_ReproducerAndPriorInvariantsRunAndPass(t *testing.T) {`

```text
// TestC1292_003_ReproducerAndPriorInvariantsRunAndPass requires the in-package
// reproducer for 1290-D2 to be tree-resident and executing (the cycle-1285
// lesson: a red reproducer minted and abandoned in the same cycle protects
// nothing), AND the pre-existing 1287/1290 invariants to still pass. Greening
// 001 by weakening the transactional or degraded-arm locks fails here.
```

### `go/acs/cycle1292/predicates_test.go:210` — above `"TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems",`

```text
// cycle-1292 reproducer (1290-D2)
```

### `go/acs/cycle1292/predicates_test.go:223` — above `func TestC1292_004_DeferralClaimIsBackedByALoadableInboxItem(t *testing.T) {`

```text
// TestC1292_004_DeferralClaimIsBackedByALoadableInboxItem is the predicate for
// 1290-D1. It drives the REAL consumer — inboxbatch.LoadDir, the loader the
// triage path uses — rather than stat-ing a filename: the defect is that a
// documented deferral is unbacked, and an item the loader drops or parses into an
// empty shell backs nothing (the cycle-1190 dropped-field shape).
```
