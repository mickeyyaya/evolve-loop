# Comment history: `acs/cycle1287`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1287/predicates_test.go:3` — above `package cycle1287`

```text
// Package cycle1287 materialises the cycle-1287 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `continuation-defect-ledger`):
//
//   - land-continuation-defect-ledger          → land the ledger/inbox/closure work
//     WITHOUT regressing the two main-side fixes the branch point predates
//   - batch-integrity-review-doc-closure-crossref → the governed docs must pass the
//     closure-citation gate they describe
//
// What is actually at risk. The ledger mechanism itself is already present and
// green in this worktree (predicates 003/004 pin that it stays so). The landing
// RISK is base drift: this lane forked at 9b129565, and `main` has since moved to
// a57e9ec4 carrying two fixes this tree does not have —
//
//	go/internal/phases/retro/retro.go       stale-worktree guard (gobridge.IsDir)
//	                                        + retro_stale_worktree_test.go
//	go/internal/router/router.go            MintSpec.Description/WhenToUse (ADR-0038)
//	                                        + mintspec_metadata_test.go
//
// A landing that takes this tree's version of those regions verbatim silently
// reintroduces the 1255-D1 stale-worktree CRITICAL and drops the ADR-0038 SELECT
// metadata wire contract, deleting both regression tests on the way. Predicates
// 001/002 are the RED that only a correctly-resolved sync can green.
//
// Predicate strategy — every predicate EXERCISES the system (runs the production
// regression test, round-trips the production type through encoding/json, builds
// the tree, or drives the production gate function); none asserts "source file
// contains string X" as its load-bearing check (the cycle-85 degenerate-predicate
// ban). Subprocess predicates are narrowed to ONE named package with an explicit
// -run expression, per the flaky-predicate-shape rules, and each guards against
// go test's exit-0-on-no-matching-test trap.
```

### `go/acs/cycle1287/predicates_test.go:79` — above `func TestC1287_001_RetroStaleWorktreeCriticalSurvivesTheLanding(t *testing.T) {`

```text
// TestC1287_001_RetroStaleWorktreeCriticalSurvivesTheLanding pins main's
// stale-worktree fix (commit 43a802d3, the 1255-D1 CRITICAL) through this
// landing. RED while this tree still carries the pre-fix `req.Worktree != "" ||
// !fleetMode(req)` guard and lacks retro_stale_worktree_test.go entirely.
//
// The assertion is the main-side regression test EXECUTING and passing here —
// not the presence of a string in retro.go — so neither re-adding the file
// without the fix nor re-adding the fix without the file can green it.
```

### `go/acs/cycle1287/predicates_test.go:96` — above `func TestC1287_002_MintSpecSelectMetadataSurvivesTheLanding(t *testing.T) {`

```text
// TestC1287_002_MintSpecSelectMetadataSurvivesTheLanding pins main's ADR-0038
// SELECT metadata contract (commit 6c4e8068) through this landing: MintSpec must
// carry Description/WhenToUse on the `description` / `when_to_use` json keys, and
// omit them when empty.
//
// Reflection rather than a struct literal deliberately: a literal referencing an
// absent field fails to COMPILE, which takes the whole predicate package down and
// hides every other predicate's verdict. Reflection keeps the failure scoped to
// this criterion. The values are driven through the real encoding/json codec in
// both directions, so this is a wire-contract exercise, not a field-name check.
```
