# Comment history: `internal/directives`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d14

### `go/internal/directives/directives.go:1` — above `package directives`

```text
// Package directives implements the runtime operator-directives cascade: a
// global layer (all loops) and a per-loop layer (keyed by runscope lane) that a
// loop re-reads at the START of each cycle and injects into every phase agent's
// prompt. The main session edits the files; all loops converge on the next cycle
// boundary without restart. See
// docs/superpowers/specs/2026-06-20-runtime-directives-cascade-design.md.
//
// This package is intentionally PURE and environment-agnostic: Load takes
// explicit paths and never reads os.Getenv/LookupEnv/Environ (the composition
// root resolves home dir + lane via Resolve and passes paths in). Directives are
// guidance prose only — they cannot weaken the binary gates, which remain the
// sole hard trust boundary.
```
