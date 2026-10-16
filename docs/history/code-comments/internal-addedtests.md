# Comment history: `internal/addedtests`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero k2

### `go/internal/addedtests/addedtests.go:1` — above `package addedtests`

```text
// Package addedtests is the ONE derivation of "which test packages does this
// tree ADD, and under which build tags must each run" — the seed the ship's
// repo-contract added-test backstop has used since #612, now shared with the
// build floor so a red added test is found while the builder still owns the
// tree. Cycle 1679 (2026-09-14): an added `//go:build acs` package was
// invisible to the floor's default-context run (no GoFiles in that context)
// and first executed by the ship gate, where its red cost repair rounds the
// builder could not apply.
```

### `go/internal/addedtests/addedtests_test.go:24` — above `func TestGroups_TagGatedAddedPackagesRunUnderTheirOwnTags(t *testing.T) {`

```text
// TestGroups_TagGatedAddedPackagesRunUnderTheirOwnTags — the cycle-1679 shape:
// an added //go:build acs package must be grouped under acs (the default
// context never sees it), an untagged added package stays in the default
// group, a modified test is not a candidate, and a requires_tmux test is
// excluded by name rather than silently dropped.
```
