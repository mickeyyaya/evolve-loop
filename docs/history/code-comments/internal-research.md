# Comment history: `internal/research`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/research/filekb_recall_test.go:11` — above `func recallCorpus(t *testing.T, n int) string {`

```text
// filekb_recall_test.go — the policy-resolved recall bound (cycle-1494,
// `sleep-time-kb-consolidation`).
```

### `go/internal/research/kb.go:65` — above `func SearchPathsFromEnv(cfg policy.PathsConfig) []string {`

```text
// SearchPathsFromEnv returns KB search paths from the policy config,
// falling back to the documented default when cfg.KBSearchPaths is empty.
// Replaced EVOLVE_KB_SEARCH_PATHS env read (cycle-17).
```

### `go/internal/research/research_adversarial_test.go:3` — above `import (`

```text
// research_adversarial_test.go — cycle-281 test amplification.
// Targets: SearchPathsFromEnv (0%), Digest (75%), firstSentence (75%),
// SplitSearchPaths edge cases, and Query.terms() deduplication.
```
