# Comment history: `internal/triagedecision`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d14

### `go/internal/triagedecision/derive.go:1` — above `package triagedecision`

```text
// Package triagedecision is the one reader of triage's report as a decision (ADR-0106 H1). The report is the
// phase's primary deliverable and states the commitment in prose sections. Derive is the strict reading the
// host writes before any judge when the phase left the decision absent; Project is the lenient reading ship
// writes for the inbox lifecycle. Nothing the report does not state is written: floors, skipped items and an
// undeclared footprint are left to their readers' own fallbacks.
```

### `go/internal/triagedecision/derive_test.go:9` — above `const report1707 = '<!-- challenge-token: 2bb098b9e2c61eb0 -->`

```text
// report1707 is cycle 1707's triage-report.md, whose decision file the phase never wrote.
```

### `go/internal/triagedecision/derive_test.go:246` — above `func TestDerive_ReadsAnEmptyOptionalBucketAsNoCards(t *testing.T) {`

```text
// An optional bucket left empty states no cards: cycle 1713's triage wrote `## superseded` with nothing
// under it, the strict reader declined, and the gate rejected a decision the report already carried.
```
