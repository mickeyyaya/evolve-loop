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

## comment-zero d9

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
// It parses; the only policy it owns is the documented precedence (Committed)
// and what counts as triage ANSWERING for an id it did not commit
// (DispositionsFrom — F30).
```

### `go/internal/committedset/committedset.go:169` — above `func DispositionsFrom(body []byte) []Disposition {`

```text
// DispositionsFrom returns every id a triage decision answered for, one entry
// per id, the most severe bucket first: escalate_block → skip_rejected →
// skip_shipped → dropped (an integrity escalation is never masked by a drop's
// reason). An answer is a statement a reader can act on (F30, cycle 1682), so
// every answer carries evidence: an escalation its reason, else its
// fail_count, else an explicit "escalated without a stated reason" (an
// escalation is still a disposition); a rejection triage's report of the
// rejected-dir match; a skip its sha; a drop its reason — a skip without a sha
// or a drop without a reason answers for nothing. A DEFERRAL is never an answer: cycle 1623's triage narrated a claim
// failure as a deferral, so a deferred id is still owed work. A malformed
// decision answers for nothing.
```

### `go/internal/committedset/committedset_test.go:30` — above `func TestCommitted_Precedence(t *testing.T) {`

```text
// TestCommitted_Precedence pins the one answer every consumer projects from.
// The shapes are the ones real cycles leave on disk: runtime cycle-1621 had a
// lane pin naming two members while triage's top_n named one, and 17 of the
// last 20 runtime cycles carried a pin — so a top_n-only reader under-reports
// on the dominant shape.
```

### `go/internal/committedset/dispositions_test.go:10` — above `func TestDispositions_OnlyAnsweringBucketsAnswer(t *testing.T) {`

```text
// TestDispositions_OnlyAnsweringBucketsAnswer pins which of triage's
// non-commitment buckets ANSWER for an id (F30, cycle 1682): a lane scoped to
// one item whose triage dropped it with a reason did its job, and must end as
// planned no-work rather than a claim failure. Deferral is deliberately NOT an
// answer — cycle 1623's triage narrated a claim failure as a deferral
// (empty_commitment_claimable_test.go), so a deferred id stays claimable work.
```

### `go/internal/committedset/dispositions_test.go:44` — above `func TestDispositions_TheMostSevereBucketWinsAndNothingIsFabricated(t *testing.T) {`

```text
// TestDispositions_TheMostSevereBucketWinsAndNothingIsFabricated: an id named
// in two buckets is reported once, by its most severe bucket — a drop's
// "already-shipped" reason never masks a fraudulent-commit escalation (F30
// architecture review m1) — and an absent or malformed decision answers for
// nothing: this package never fabricates.
```
