# Comment history: `pkg/version`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/pkg/version/version.go:36` — above `func Commit() string {`

```text
// Commit returns the raw embedded build-commit (short sha) used for
// provenance checks (ADR-0065: a binary is legitimate when its build-commit
// is an ancestor of HEAD). Returns "" when the binary was not stamped (the
// `go run` / `go test`-without-VCS path), which callers treat as
// unverifiable provenance. Preference: ldflag `commit`, else BuildInfo
// vcs.revision.
```

### `go/pkg/version/version_test.go:221` — above `func TestComposeCommit(t *testing.T) {`

```text
// composeCommit resolves the raw build-commit (short sha) used for provenance
// (ADR-0065): ldflag-injected `commit` wins; else BuildInfo vcs.revision; else
// "". Truncated/trimmed via shortSHA. Drives every branch deterministically.
```
