# Comment history: `internal/rawgitratchet`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/rawgitratchet/apicover_named_test.go:46` — above `func TestBoundTestFiles_BindsTrackedOnly(t *testing.T) {`

```text
// TestBoundTestFiles_BindsTrackedOnly names BoundTestFiles: in a work tree a
// staged test file is bound, while an untracked one, a non-test file and one
// under testdata or a dot directory are not (ADR-0084 I1).
```

### `go/internal/rawgitratchet/rawgitratchet.go:1` — above `package rawgitratchet`

```text
// Package rawgitratchet keeps new raw git fixtures out of the module's tests.
// A test outside internal/gittest that builds its own git repository fails the
// ratchet unless its file is listed in baseline.json, and a listed file's
// count of init sites may only shrink until its entry disappears. The fixture
// home is internal/gittest, whose repos keep git's detached maintenance child
// from racing t.TempDir() removal.
//
// The scan binds the module's git-tracked test files, index-staged included,
// so an untracked file never reds it (ADR-0084 I1). When git cannot answer —
// the module is not in a work tree — every on-disk test file is bound instead:
// the strict direction, never bind-none.
```
