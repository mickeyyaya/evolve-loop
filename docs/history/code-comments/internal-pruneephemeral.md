# Comment history: `internal/pruneephemeral`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d11

### `go/internal/pruneephemeral/pruneephemeral_test.go:88` — above `for _, name := range []string{"cycle-1", "cycle-3"} {`

```text
// cycle-1 + cycle-3 should be gone; cycle-2 should remain.
```

### `go/internal/pruneephemeral/pruneephemeral_test.go:311` — above `bare := filepath.Join(repo, ".evolve", "runs", "cycle-2")`

```text
// cycle-2 exists but has NO .ephemeral child (just a plain file).
```
