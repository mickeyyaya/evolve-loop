# Comment history: `acs/cycle3`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle3/predicates_test.go:3` — above `package cycle3`

```text
// Package cycle3 materializes the cycle-3 acceptance criteria for:
//
//   - cliadmit-cross-process-admission: Slice 4 of the concurrency-arch-slices
//     campaign. New leaf pkg internal/cliadmit — cross-process LLM-CLI admission
//     control via flock'd holder-set JSON, TTL pruning, and Acquire/release hook
//     in internal/bridge/driver_tmux_repl.go.
```

### `go/acs/cycle3/predicates_test.go:21` — above `func TestC3_001_CliadmitPackageExistsAndTracked(t *testing.T) {`

```text
// TestC3_001_CliadmitPackageExistsAndTracked asserts that
// go/internal/cliadmit/cliadmit.go was created in the worktree and is
// git-tracked. A gitignored file is silently dropped at ship (cycle-93 lesson).
```

### `go/acs/cycle3/predicates_test.go:239` — above `func TestC3_010_ApiCoverEnforceContainsCliadmit(t *testing.T) {`

```text
// TestC3_010_ApiCoverEnforceContainsCliadmit asserts ./internal/cliadmit is
// enrolled in go/.apicover-enforce. Enrollment is mandatory for every new
// internal package (TestApicoverEnforce_CoversEveryInternalPackage gate,
// cycle-131 lesson: a new pkg not enrolled fails the completeness invariant
// at ship).
//
// acs-predicate: config-check — enrollment verification is inherently a
// file-presence check; the behavioral gate is TestC3_011.
```
