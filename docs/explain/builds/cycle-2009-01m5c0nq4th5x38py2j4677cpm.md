# Build Explanation — Cycle 2009

## Build Binding
- Cycle: 2009
- Base SHA: c148fb5b3869b5db7261251cc098eaf6c7beb95f

## Summary
The flags tests now ignore a lane's EVOLVE_WORKTREE_ROOT, so `runFlags generate` writes only into the temp project of the test.

## Rationale
`runFlags` resolves the flags document from EVOLVE_WORKTREE_ROOT before EVOLVE_PROJECT_ROOT. A lane exports that variable, so the tests wrote into the lane's own document and failed. One shared seed helper clears the variable, which fixes every caller at once.

## Changed Areas
- `go/cmd/evolve/cmd_flags_test.go` — `seedFlagsProject` clears EVOLVE_WORKTREE_ROOT through `t.Setenv`, so the value returns after each test.
- `go/acs/cycle2009/predicates_test.go` — predicates of the TDD phase that prove the three acceptance criteria.

## Design Decisions
The fix lives in the test helper, not in production code. Production resolution order stays as the worktree-root test requires. Rejected: editing each test to set the variable, because four callers would repeat it.

## Verification
`EVOLVE_WORKTREE_ROOT=/tmp/elsewhere go test -run 'TestFlags|TestSplice' ./cmd/evolve/` and the acssuite predicate-env test pass. The cycle2009 ACS predicates pass.

## Compatibility
No production code changes. `TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot` still sets the variable itself after seeding.

## Limitations
Other tests that read EVOLVE_WORKTREE_ROOT are out of scope.
