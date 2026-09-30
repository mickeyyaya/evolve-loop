# Comment history: `internal/committedset`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/committedset/committedset.go:1` — above `package committedset`

```text
// Package committedset owns the ONE answer to "which tasks did this cycle
// commit to?" and the on-disk shapes it is read from.
//
// It exists because that question had four independent readers that disagreed
// on live cycles: core.ContractTaskIDs (lane pin, else decision top_n, minus
// deferred), core.BoundTaskIDs (decision top_n only), inboxmover.CommittedIDs
// (top_n ∪ skip_shipped) and, briefly, the cycle dossier (top_n only). On
// runtime cycle-1621 the lane pin named two members while top_n named one, so
// a top_n-only reader under-reported the commitment — and 17 of the last 20
// runtime cycles carried a lane pin, so that was the DOMINANT shape, not an
// edge case. A record that reports a commitment it did not read is worse than
// one that reports none.
//
// This package is a LEAF (stdlib only) so every consumer can project from it:
// internal/core owns the orchestrator and internal/dossier cannot import core.
// It parses; it does not decide policy beyond the documented precedence.
```
