# Comment history: `acs/cycle1529`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1529/predicates_test.go:3` — above `package cycle1529`

```text
// Package cycle1529 carries the cycle-1529 ACS predicates.
//
// Task: close-completion-contract-cancel-parity-stale-item — retire the stale
// inbox item `completion-contract-cancel-parity` as not-observed/already-fixed.
// Scout proved the defect it worried about was fixed and test-locked by the
// `withFinalPoll` generalization (go/internal/bridge/completion.go), pinned by
// go/internal/bridge/completion_cancel_parity_test.go. The item's own
// acceptance criteria say "close as not-observed if none", so the cycle's work
// is a doc-only closure — and the two non-regression predicates below exist to
// prove the closure stayed doc-only.
```

### `go/acs/cycle1529/predicates_test.go:119` — above `if !gitTracked(root, rel) {`

```text
// cycle-93 lesson: disk presence without tracking ships as nothing.
```

### `go/acs/cycle1529/predicates_test.go:151` — above `const (`

```text
// Cycle 1529's shipped closure: its ship commit on main and that commit's
// parent. Their diff is the closure this package vouches for.
```

### `go/acs/cycle1529/predicates_test.go:158` — above `func TestC1529_004_ClosureStaysDocOnly(t *testing.T) {`

```text
// TestC1529_004_ClosureStaysDocOnly is the negative/scope axis. The task is a
// closure, explicitly NOT the hardening the stale item scoped: touching
// go/internal/bridge this cycle means the lane re-litigated an already-fixed
// defect without a RED test for it. It diffs cycle 1529's own base..ship range,
// never the live `main` ref, so later bridge work on either side of main
// cannot fail it.
```
