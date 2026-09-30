# Comment history: `acs/cycle1720`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1720/predicates_test.go:3` — above `package cycle1720`

```text
// Package cycle1720 materializes the acceptance criteria for this fleet lane's
// two triage-committed tasks under inbox id triage-unified-solution-synthesis:
//
//   - transactional-unified-consume → the ship's in-commit consumption closes a
//     VALIDATED unified_commitment all-or-nothing: if any member cannot close,
//     every member stays pickable and one WARN names the commitment and the
//     failing member; ordinary ids keep the per-item fail-open contract.
//   - unified-commitment-validation-tests → processUnifiedCommitment gets direct
//     unit tests for its five branches, and those tests must DETECT a
//     regression in the branch each one names.
//
// consumeCommittedItems and processUnifiedCommitment are unexported, so the
// behavioral proofs are named in-package tests run as one-package `go test`
// subprocesses (the cycle-1507 shape): each predicate asserts the exact
// `--- PASS:` lines, never the exit code (`go test -run` over a pattern that
// matches nothing exits 0). The consume tests are TDD-authored and frozen;
// 004 drives the real cycle ship (shipFromWorktree over a real git worktree),
// so the landing commit itself is the evidence. For the test-only task the
// predicate is mutation-based (007): each branch of the production code is
// broken through `go test -overlay` — no byte of the tree changes — and the
// Builder's subtest for that branch must go RED.
//
// Flaky-shape hygiene: every subprocess names ONE package and narrows with
// -run, no wall-clock bounds, no literal PIDs, every git call is -C rooted.
```

### `go/acs/cycle1720/predicates_test.go:269` — above `func TestC1720_008_CycleTestFilesAreGitTracked(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// ship-tree tracking (cycle-93 / cycle-1623 M1)
// ---------------------------------------------------------------------------
```
