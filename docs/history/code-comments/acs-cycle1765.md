# Comment history: `acs/cycle1765`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1765/predicates_test.go:32` — above `baseCommit   = "156ee9aba02a05829d81d7f783f456bfd86c9420"`

```text
// baseCommit is the merge-base of this branch with origin/main at TDD
// authoring time (2026-09-30): the boundary between "main's state" and
// "this lane's diff", not an arbitrary point in this branch's own history.
```

### `go/acs/cycle1765/predicates_test.go:60` — above `var exemptPrefixes = []string{`

```text
// exemptPrefixes are pipeline-required artifact paths this lane's diff
// against baseCommit legitimately touches beyond the three target packages:
// this cycle's own predicate package, the permanent eval file the AC
// authority lives in, build/closeout explanations, the prior attempt's
// archived predicate packages (A2 continuation-retirement), and knowledge
// base cycle summaries. A deny-list, not an allow-list of the task's own
// packages: cycle-1761's allow-list style scope fence went red on its own
// pipeline-required explanation/eval writes (flaky-predicate-shape lesson).
```
