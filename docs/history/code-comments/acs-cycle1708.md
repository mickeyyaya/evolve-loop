# Comment history: `acs/cycle1708`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1708/predicates_test.go:3` — above `package cycle1708`

```text
// Package cycle1708 materialises the acceptance criteria for
// lineage-datestamp-normalization: LineageKey must additionally strip
// calendar-year-shaped date runs so same-line dated snapshots
// (gpt-4o-2024-08-06, gpt-4o-2024-11-20) share a lineage bucket and
// PromoteLatest can pick the newer one, while size/tag digit suffixes
// (:8b, :70b, 32b) stay untouched and the reuse-gate decisionVersion ratchet
// is bumped for the semantics change. Cycle 1707 built and committed this
// same fix (salvage snapshot e54e8500) but did not ship it; this cycle's
// predicates re-materialise the same acceptance criteria against the
// worktree's current state so audit/ship gate this cycle.
```

### `go/acs/cycle1708/predicates_test.go:22` — above `func TestC1708_001_DatedSnapshotsShareLineageKey(t *testing.T) {`

```text
// TestC1708_001_DatedSnapshotsShareLineageKey is AC1: gpt-4o-2024-08-06 and
// gpt-4o-2024-11-20 are the same model line at different dated snapshots and
// must bucket under the same LineageKey so PromoteLatest can act on them.
```
