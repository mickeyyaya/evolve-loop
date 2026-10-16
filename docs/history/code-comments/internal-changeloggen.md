# Comment history: `internal/changeloggen`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d10

### `go/internal/changeloggen/changeloggen.go:113` — above `func appendBucket(sb *strings.Builder, heading string, items []string) {`

```text
// appendBucket renders one `### <heading>` section. Identical items collapse to
// a single bullet, keeping first-occurrence order: a merge can make one commit
// reachable by two paths, which rendered the same bullet twice (CHANGELOG.md
// [22.13.1], both ending `(#406)`). Dedup is per-bucket, so the same text
// legitimately classified into two sections still renders once in each.
```

### `go/internal/changeloggen/changeloggen_test.go:376` — above `func TestRenderEntry_DedupsIdenticalBulletsWithinBucket(t *testing.T) {`

```text
// TestRenderEntry_DedupsIdenticalBulletsWithinBucket is the regression test for
// the verbatim-duplicated `(#406)` bullet shipped in CHANGELOG.md [22.13.1]: a
// merge made one commit reachable by two paths, so ClassifyAll produced the same
// rendered item twice inside one bucket.
```
