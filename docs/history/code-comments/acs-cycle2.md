# Comment history: `acs/cycle2`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle2/predicates_test.go:3` — above `package cycle2`

```text
// Package cycle2 materializes the cycle-2 acceptance criteria for:
//
//   - sessionreaper-orphan-reap: Tier-3 liveness orphan reaper (Slice 3,
//     concurrency-arch-slices campaign). New leaf pkg internal/sessionreaper
//     with ReapOrphans function, replacement of the looppreflight glob-WARN,
//     and `evolve swarm reap-orphans` CLI operator backstop.
```

### `go/acs/cycle2/predicates_test.go:22` — above `func TestC2_001_SessionreaperPackageExistsAndTracked(t *testing.T) {`

```text
// TestC2_001_SessionreaperPackageExistsAndTracked asserts that
// go/internal/sessionreaper/sessionreaper.go was created in the worktree
// and is git-tracked. A gitignored file is silently dropped at ship
// (cycle-93 lesson).
```

### `go/acs/cycle2/predicates_test.go:211` — above `func TestC2_010_ApiCoverEnforceContainsSessionreaper(t *testing.T) {`

```text
// TestC2_010_ApiCoverEnforceContainsSessionreaper asserts ./internal/sessionreaper
// is enrolled in go/.apicover-enforce. Enrollment is mandatory for every new
// internal package (TestApicoverEnforce_CoversEveryInternalPackage gate, cycle-131
// lesson: a new pkg not enrolled fails the completeness invariant at ship).
//
// acs-predicate: config-check — enrollment verification is inherently a
// file-presence check; the behavioral gate is TestC2_011.
```
