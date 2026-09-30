# Comment history: `acs/cycle1304`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1304/predicates_test.go:3` — above `package cycle1304`

```text
// Package cycle1304 holds the cycle-1304 ACS predicates.
//
// Scope: this lane is pinned to the todo-id `contract-block-cli-escalation`,
// whose code fix already landed on main in cycle-1300 (dossier 061345a4).
// The two committed tasks are queue/doc hygiene, so the system under test is
// the QUEUE STATE and the DOC ROW themselves — these artifacts are the
// deliverable, not a source-grep proxy for some other behavior. The inbox
// predicates parse the JSON payload (identity + provenance fields) rather
// than grepping for a magic string, and the README predicates pin the row's
// semantics plus a no-collateral-damage guard so "delete the row" cannot pass.
```

### `go/acs/cycle1304/predicates_test.go:28` — above `wantItemID        = "contract-block-cli-escalation"`

```text
// Identity of that item's payload — the pair that distinguishes it from
// the superseded 2026-07-30 instance already sitting in consumed/.
```

### `go/acs/cycle1304/predicates_test.go:122` — above `func TestC1304_002_InboxItemLandsInConsumed(t *testing.T) {`

```text
// TestC1304_002_InboxItemLandsInConsumed asserts the SAME item (matched by the
// id + created_at identity pair, not by filename) now lives under consumed/.
// Paired with 001 this makes a delete-instead-of-move indistinguishable from a
// failure. RED today: nothing in consumed/ carries created_at 2026-08-04.
```

### `go/acs/cycle1304/predicates_test.go:143` — above `func TestC1304_003_ConsumedItemCitesLanding(t *testing.T) {`

```text
// TestC1304_003_ConsumedItemCitesLanding asserts the consumed item carries a
// consumed_by provenance field naming the cycle-1300 landing evidence, matching
// the convention the 2026-07-30 instance already established. Without this the
// move is an unexplained deletion. RED today: the item is not in consumed/ at all.
```

### `go/acs/cycle1304/predicates_test.go:170` — above `func TestC1304_004_PriorConsumedInstanceUntouched(t *testing.T) {`

```text
// TestC1304_004_PriorConsumedInstanceUntouched is the negative/collateral guard:
// the superseded 2026-07-30 instance must survive with its own provenance intact.
// A move that clobbers the destination filename would trip this.
```
