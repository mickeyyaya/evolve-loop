# Comment history: `acs/cycle1707`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1707/predicates_test.go:3` — above `package cycle1707`

```text
// Package cycle1707 materialises the acceptance criteria for
// lineage-datestamp-normalization: LineageKey must additionally strip
// calendar-year-shaped date runs so same-line dated snapshots
// (gpt-4o-2024-08-06, gpt-4o-2024-11-20) share a lineage bucket and
// PromoteLatest can pick the newer one, while size/tag digit suffixes
// (:8b, :70b, 32b) stay untouched and the reuse-gate decisionVersion ratchet
// is bumped for the semantics change.
```

### `go/acs/cycle1707/predicates_test.go:19` — above `func TestC1707_001_DatedSnapshotsShareLineageKey(t *testing.T) {`

```text
// TestC1707_001_DatedSnapshotsShareLineageKey is AC1: gpt-4o-2024-08-06 and
// gpt-4o-2024-11-20 are the same model line at different dated snapshots and
// must bucket under the same LineageKey so PromoteLatest can act on them.
```
