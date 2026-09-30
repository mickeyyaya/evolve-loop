# Comment history: `internal/scopedelta`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/scopedelta/apicover_named_test.go:3` — above `import "testing"`

```text
// apicover_named_test.go — names AND exercises the exports the behavioural
// suite reaches only indirectly (export-naming floor, ADR-0069). Not ceremony:
// each test below pins a contract a caller depends on, and the repo has burned
// four CI reds this month on new exports that had behavioural coverage from a
// neighbouring package but no test naming them here.
```

### `go/internal/scopedelta/scopedelta.go:62` — above `ClassBoundary Class = "boundary"`

```text
// ClassBoundary — a protected, operator-owned surface (ADR-0074). Policy,
// not merit: refused whatever the justification, and preserved anyway.
```

### `go/internal/scopedelta/scopedelta.go:89` — above `Protected []string`

```text
// Protected surfaces are operator-owned; a lane may not edit them
// regardless of merit (ADR-0074). Directory prefixes end in "/".
```
