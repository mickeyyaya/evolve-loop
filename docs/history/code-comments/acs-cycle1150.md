# Comment history: `acs/cycle1150`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1150/predicates_test.go:3` — above `package cycle1150`

```text
// Package cycle1150 materialises the cycle-1150 acceptance criteria for this
// lane's single triage-committed top_n task:
//
//	wire-docsfloor-verify-cli (M) — wire the ADR-0077 blocking-grade classifier
//	    `deliverable.VerifyBuildWithChangedPaths` (added cycle-1144, ZERO
//	    production callers) into `evolve phase verify build`, the self-check
//	    every phase prompt's Deliverable Contract tells the agent to run before
//	    declaring done. The changed-path set comes from a newly exported
//	    `core.ChangedWorktreePaths` — a projection of the existing unexported
//	    derivation the host-side docs-floor reviewer already uses, NOT a second
//	    implementation (ADR-0034 no-drift invariant).
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1144
// precedent). Each predicate shells `go test -run` over the RED contract tests
// authored this cycle, every one of which drives the REAL CLI entry point
// (`runPhaseVerify`) over a REAL git worktree and asserts on exit codes and
// stderr, or calls the real `core.ChangedWorktreePaths`. None is a source-grep
// of production code (the cycle-85 degenerate-predicate ban); C1150_004 carries
// one supplementary source assertion, but its load-bearing half is the
// subprocess run.
//
// RED now: `evolve phase verify build` calls `deliverable.VerifyWithStage`
// (phase_verify.go:71), which never sees the diff — so an architecture-class
// build with no docs delta exits 0; and `core.ChangedWorktreePaths` does not
// exist, so internal/core's test package fails to compile.
```

### `go/acs/cycle1150/predicates_test.go:98` — above `func TestC1150_004_ChangedPathsAreOneSourceWithAProjection(t *testing.T) {`

```text
// TestC1150_004_ChangedPathsAreOneSourceWithAProjection — AC5, the
// no-duplication contract. The changed-path derivation must be the EXPORTED
// wrapper over core's existing logic, and the CLI must actually call the
// ADR-0077 seam function rather than growing its own git shell-out.
//
// Load-bearing assertion is the subprocess run (it CALLS core.ChangedWorktreePaths
// and asserts on returned values); the two source assertions are supplementary
// anti-gaming checks that the projection is the one being consumed — neither
// can pass on its own, C1150_001/002 still have to go green through the real
// classifier.
```
