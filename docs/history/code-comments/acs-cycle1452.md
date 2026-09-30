# Comment history: `acs/cycle1452`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1452/predicates_test.go:3` — above `package cycle1452`

```text
// Package cycle1452 materialises the cycle-1452 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane: `consumption-rides-landing-ship`
// (weight 0.92, pipeline-repair).
//
// The defect: consuming an inbox item is a separate act from the ship that
// closes it, so forgetting is always possible. Live instance 2026-08-12 —
// schema-aligned-salvage-layer landed in #453, its item was never consumed, and
// wave cycle-1448 re-picked already-shipped work as live scope.
//
// The fix under test: a builder-authored, line-anchored `Closes-Inbox: <id>`
// marker in build-report.md, unioned into the committed set inside promoteInbox
// under the EXACT existing cycle-598 landing gate.
//
// Predicate strategy — every predicate exercises the system (the cycle-85
// degenerate-predicate ban):
//
//   - 001–002 CALL the real parser over real report bodies: 001 pins the
//     positive contract, 002 is the anti-false-positive half (prose that merely
//     mentions the marker must consume nothing). A substring-anywhere
//     implementation greens 001 and reds 002.
//   - 003–004 shell the ship package's own promoteInbox fixtures — ONE named
//     package narrowed with `-run`, per the flaky-predicate-shape rules — and
//     assert the named tests were actually SELECTED (Go's `-run` is a substring
//     match, so an unselected test is a silent pass). 003 is the consume half,
//     004 the must-NOT-consume half (unlanded ship, unmarked item).
//   - 005 is the documentation criterion: a text-presence check by nature, so it
//     carries an explicit waiver AND asserts git-TRACKING, not just disk
//     presence (cycle-93: a gitignored file is dropped at ship).
```

### `go/acs/cycle1452/predicates_test.go:95` — above `func shipFixtures(t *testing.T, pattern string, names ...string) {`

```text
// shipFixtures asserts the given -run pattern selects AND passes every named
// promoteInbox fixture in the ship package. Selection is asserted explicitly:
// Go's -run is a substring match, so a renamed or absent test would otherwise
// pass vacuously (the cycle-1446 L3 class).
```

### `go/acs/cycle1452/predicates_test.go:118` — above `func TestC1452_003_MarkedItemIsConsumedByItsOwnLandingShip(t *testing.T) {`

```text
// TestC1452_003_MarkedItemIsConsumedByItsOwnLandingShip — the wiring proof named
// in the inbox item's own `fix` field ("a fixture PASS landing an item's
// predicates must consume it transactionally"). Drives the REAL promoteInbox
// production path through the ship package's fixtures, including the
// decision-less lane shape that produced the #453 live instance.
```

### `go/acs/cycle1452/predicates_test.go:130` — above `func TestC1452_004_UnlandedOrUnmarkedLandingConsumesNothing(t *testing.T) {`

```text
// TestC1452_004_UnlandedOrUnmarkedLandingConsumesNothing — the other half of the
// same `fix` sentence ("a partial landing must NOT"). Two ways to over-consume:
// a second, weaker gate on the marker path (cycle-598 reopened), or inferring
// closure from the diff. Both are fixture-pinned here, plus the degrade case
// where build-report.md is absent entirely.
```

### `go/acs/cycle1452/predicates_test.go:143` — above `func TestC1452_005_MarkerConventionIsDocumentedAndTracked(t *testing.T) {`

```text
// TestC1452_005_MarkerConventionIsDocumentedAndTracked — Task 4. A convention no
// Builder is told about is not a mechanism, so the persona reference and the
// canonical protocol doc must both carry the marker AND the must-NOT-on-a-partial
// -landing caveat. Tracking is asserted by subprocess because a gitignored doc is
// silently dropped at ship (cycle-93).
//
// acs-predicate: config-check — a documentation-presence criterion has no
// runtime behaviour to invoke; the executable half is the git-tracking probe.
```
