# Comment history: `internal/apicover`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/apicover/diffscope_test.go:3` — above `import (`

```text
// diffscope_test.go — cycle-1048 pins: enforcement is diff-scoped. A lane owns
// the hygiene of files it touched; pre-existing debt in untouched files WARNs
// and never fails the run. Nil ChangedFilesByDir = classic behavior (CI).
```

### `go/internal/apicover/run.go:20` — above `ChangedFilesByDir map[string]map[string]bool`

```text
// ChangedFilesByDir diff-scopes enforcement (cycle-1048: four PRE-EXISTING
// false-green core symbols floor-blocked every core-touching lane). Keyed
// by the Dirs entry; the inner set holds changed file basenames in that
// package. When set, violations in files OUTSIDE the change are reported
// under a PRE-EXISTING DEBT header and do NOT count toward the Enforce
// exit code. Nil = classic behavior (CI's repo-wide run is unchanged).
```

### `go/internal/apicover/run.go:216` — above `func splitPreExisting(r Report, changed map[string]bool) (Report, []Symbol) {`

```text
// splitPreExisting removes violations whose defining file is NOT in the
// change set, returning them separately: a lane owns the hygiene of the files
// it touches, never the debt of files it didn't (cycle-1048).
```
