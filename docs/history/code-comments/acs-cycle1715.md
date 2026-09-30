# Comment history: `acs/cycle1715`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1715/predicates_test.go:3` — above `package cycle1715`

```text
// Package cycle1715 materialises the acceptance criteria for
// acs-cycle1529-doc-only-predicate-diffs-against-local-main:
// TestC1529_004_ClosureStaysDocOnly must judge cycle 1529's own closure
// commit range, not the checkout's drift from the live `main` ref.
//
// Each predicate compiles the real cycle1529 predicate package and runs
// TestC1529_004 inside a throwaway git repository that borrows this
// repository's objects through alternates, so the verdict under test is the
// predicate's own. The fixture checks out go/internal/bridge only (sparse),
// and every ref or replace object it writes stays in the fixture.
```

### `go/acs/cycle1715/predicates_test.go:28` — above `c1529Ship = "57e227c1e36f33562c922dcdf2546b160739e45d"`

```text
// c1529Ship is cycle 1529's evolve-cycle ship commit on main's
// first-parent chain; c1529Base is its parent. Their diff is the
// closure the predicate vouches for (four files, none under
// go/internal/bridge).
```

### `go/acs/cycle1715/predicates_test.go:182` — above `func TestC1715_001_LaterBridgeChangeDoesNotFailC1529Closure(t *testing.T) {`

```text
// TestC1715_001_LaterBridgeChangeDoesNotFailC1529Closure is AC2's first
// half and AC1: bridge changes that are not cycle 1529's leave its closure
// predicate green, whichever side of the `main` ref they sit on.
```

### `go/acs/cycle1715/predicates_test.go:226` — above `func TestC1715_002_BridgeChangeInsideC1529RangeStillFails(t *testing.T) {`

```text
// TestC1715_002_BridgeChangeInsideC1529RangeStillFails is AC2's second half
// and AC1's negative axis. A replace object makes cycle 1529's ship commit
// carry a bridge source, with no drift from main: the predicate must pass
// on the real range and fail on the altered one, naming the file.
```
