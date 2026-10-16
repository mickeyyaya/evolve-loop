# Comment history: `internal/resolvellm`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/resolvellm/resolvellm.go:30` — above `ModelTier string`

```text
// ModelTier is the canonical abstract tier ("fast" | "balanced" | "deep").
// Legacy values ("haiku" | "sonnet" | "opus") are accepted for one release
// via the realizer fallback ladder + parseManifest v1 shim; see ADR-0022.
```
