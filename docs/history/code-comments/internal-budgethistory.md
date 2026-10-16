# Comment history: `internal/budgethistory`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d7

### `go/internal/budgethistory/apicover_named_test.go:3` — above `import "testing"`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported budgethistory symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.
```

### `go/internal/budgethistory/budgethistory_tokens_test.go:63` — above `writeRunWithTokens(t, root, 20, 600_000, []cyclestate.TokenUsage{`

```text
// cycle 20: 100+10+0+0 = 110 in/out, +0 cache ⇒ gross 110... split across two
// phases below to prove the roll-up sums PHASES, not just reads the first.
```
