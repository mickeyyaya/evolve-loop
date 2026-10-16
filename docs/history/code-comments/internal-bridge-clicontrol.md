# Comment history: `internal/bridge/clicontrol`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/bridge/clicontrol/usage_test.go:24` — above `const realCodexStatusPane = '╭────────────────────────────────────────────────────────╮`

```text
// realCodexStatusPane — codex 0.147.0, captured live 2026-08-29. Note the
// shape traps: a unicode bar before the figure, TWO "Weekly limit" rows (the
// account's, and one nested under a per-model heading), and a bodiless
// "GPT-5.3-Codex-Spark limit:" heading with no figure at all.
```

### `go/internal/bridge/clicontrol/usage_test.go:39` — above `const realClaudeUsagePane = '▝▜██████▀  Opus 5 (1M context) with xhigh effort · Claude Max`

```text
// realClaudeUsagePane — the figure claude reports about itself, captured live
// 2026-08-28 (Claude Code v2.1.248). claude states usage CONSUMED; codex states
// remaining. Normalising that difference is exactly what a caller should not
// have to know.
```
