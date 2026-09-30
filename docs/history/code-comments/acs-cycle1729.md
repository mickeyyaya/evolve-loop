# Comment history: `acs/cycle1729`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1729/predicates_test.go:3` — above `package cycle1729`

```text
// Package cycle1729 materialises raw-git-fixture-ratchet: a repo-contract test
// fails on a new raw git init in a test outside internal/gittest, and the
// existing call sites are listed and may only shrink.
//
// Pinned contract:
//
//   - the ratchet is the default (untagged) test suite of go/internal/rawgitratchet,
//     and its list of existing call sites is go/internal/rawgitratchet/baseline.json;
//   - it binds the module's git-TRACKED test files (index-staged counts as
//     tracked), so an untracked file never reds it (ADR-0084 I1), and it binds
//     every on-disk test file when the module is not in a git work tree;
//   - a violation fails a named test whose output names the file by its
//     module-relative path;
//   - the list is per file and per call site: a new site in a listed file fails,
//     and a migrated file must leave the list;
//   - a ship.repoContractPackages entry reaches the package, and the package is
//     apicover-clean and enrolled in go/.apicover-enforce.
//
// Every negative predicate runs the real ratchet against a copy of the module
// in t.TempDir(); the worktree is never modified.
```
