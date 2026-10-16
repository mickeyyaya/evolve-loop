# Comment history: `internal/modelcatalog`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h1

### `go/internal/modelcatalog/refresh_test.go:73` — above `func TestMergeFallbacks_PreservesTierFallbacks(t *testing.T) {`

```text
// TestMergeFallbacks_PreservesTierFallbacks is the cycle-899 regression test:
// `evolve models refresh` rebuilds the catalog wholesale, and before this fix
// any operator-authored tier_fallbacks (e.g. deep→claude-fable-5) was silently
// destroyed by the rewrite.
```

### `go/internal/modelcatalog/store_writefail_test.go:11` — above `type fakeTempFile struct {`

```text
// ---------------------------------------------------------------------------
// RED tests for the temp-file Write/Sync/Close failure branches of store.Write.
//
// store.Write at baseline is 66.7% covered. The mkdir / CreateTemp / Rename
// error exits are already exercised by store_errors_test.go via filesystem
// tricks. The three REMAINING dark branches operate on the *temp file* after it
// is opened — tmp.Write, tmp.Sync, tmp.Close — and there is NO filesystem trick
// that makes a freshly-created regular file fail those operations portably. They
// require an injectable seam over os.CreateTemp.
//
// These tests reference a package-internal seam that Builder must add to
// store.go (see test-report.md "Handoff to Builder"):
//
//	type tempFile interface {
//	    io.Writer
//	    Sync() error
//	    Close() error
//	    Name() string
//	}
//	var createTemp = func(dir, pattern string) (tempFile, error) {
//	    f, err := os.CreateTemp(dir, pattern)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return f, nil
//	}
//
// and Write must call createTemp(...) instead of os.CreateTemp(...). Until that
// seam exists this file does not compile, which is the RED state (the TDD skill
// permits compile-failure as RED). Each test verifies the full contract of an
// error branch: the wrapped error IS surfaced AND the temp file is cleaned up
// (no torn cache, no leaked .tmp) — that cleanup is the atomic-write invariant,
// not an incidental detail.
// ---------------------------------------------------------------------------
```
