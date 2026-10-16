# Comment history: `internal/verifylock`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/verifylock/verifylock.go:1` — above `package verifylock`

```text
// Package verifylock serializes the host's EXPENSIVE go-test verification
// runs (ADR-0080 P1). Fleet lanes are separate OS processes that each shell
// full package suites during EGPS and the build handoff floor; running them
// concurrently oversubscribes one host and turns long suites into false reds
// (batch-16: TouchedPackagesStayGreen red on three lanes, green in the
// preserved worktree — the halt's entire fingerprint). One flock, hub-
// resident so every lane of a repo shares it; LLM phases stay parallel —
// only verification is single-flight.
```

### `go/internal/verifylock/verifylock_test.go:3` — above `import (`

```text
// verifylock_test.go — ADR-0080 P1: expensive go-test verification runs
// (EGPS suite execution, build-floor coverage) are HOST-wide single-flight.
// Batch-16 halt ground truth: TouchedPackagesStayGreen red-lined three lanes
// (1166/1167/1169) while the same suite is GREEN in the preserved worktree —
// two lanes + EGPS oversubscribing one host turned a 43s suite into false
// reds. Verification correctness outranks wave throughput.
```
