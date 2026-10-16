# Comment history: `internal/installer`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D4

### `go/internal/installer/installer.go:70` — above `var referenceDocs = []string{`

```text
// referenceDocs are the docs/reference/*.md files ci.yml step 6 required to
// exist (paths post the v8.48.0 docs/ reorg; island-model.md was removed in the
// skill-publishing cleanup, so it is not required).
```

### `go/internal/installer/installer_test.go:459` — above `if !strings.Contains(UsageLine, "/evo:loop") {`

```text
// Post-rename (2026-06-24): the installer prints the live /evo:loop command,
// never the dead /evolve-loop one. Locks the user-facing display string.
```
