# Comment history: `acs/cycle1433`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1433/predicates_test.go:3` — above `package cycle1433`

```text
// Package cycle1433 encodes the cycle-1433 ACS predicates for the two tasks
// derived from inbox item `ledger-fleet-concurrency-chain`:
//
//   - ledger-anchor-reject-nonunique-seq — `FileLedger.Anchor` binds the FIRST
//     line whose entry_seq matches (anchor.go:173 self-documents the sibling
//     ambiguity it does not reject), so `evolve ledger anchor <seq>` can silently
//     bind an EARLIER sibling and regress the epoch anchor backward.
//   - ledger-rebaseline-command — no `evolve ledger rebaseline` exists anywhere in
//     go/, which is why the console-plane ledger (~180+ dense breaks) was left
//     broken rather than repaired by 180 sequential `anchor` calls.
//
// Predicate shape notes:
//   - Every predicate drives the REAL production entry point (the compiled
//     `evolve` binary, `go/cmd/evolve`) against a synthetic ledger in t.TempDir().
//     No predicate greps source for a magic string as a load-bearing assertion
//     (the cycle-85 ban), and none calls the ledger package directly — a seam
//     reachable only from a unit test is dead code (the wiring-proof rule).
//   - No `./...` sweep, no whole-package `go test`, no wall-clock bound, no
//     literal PID, no bare `git`, no load generator (the flaky-shape rules that
//     produced the 1173/1175/1178 false-REDs). The single subprocess cost is one
//     `go build ./cmd/evolve`, done once for the whole package.
//   - The rebaseline NEGATIVE predicate (005) explicitly refuses to pass on
//     "unknown subcommand": without that guard it would green vacuously today,
//     while the command does not exist at all.
```

### `go/acs/cycle1433/predicates_test.go:378` — above `after, err := os.ReadFile(path)`

```text
// Non-destructive: the pre-rebaseline bytes must remain a byte-identical
// PREFIX on disk. Truncating or rewriting history would "green" the chain by
// destroying the auditable record, which is the outcome ADR-0048 rejects.
```
