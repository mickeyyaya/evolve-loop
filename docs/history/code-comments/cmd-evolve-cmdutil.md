# Comment history: `cmd/evolve/cmdutil`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/cmd/evolve/cmdutil/cmdutil.go:39` — above `func EnvOrCwd(env string) string {`

```text
// EnvOrCwd returns the absolute root named by environment variable env when it
// is set — absolutized via paths.AbsoluteRoot so downstream paths are
// cwd-independent (the cycle-119 fix class) — otherwise the current working
// directory (already absolute), otherwise ".". Extracted from cmd_subagent.go
// so the decomposed internal/cli/* command groups share one implementation.
```

### `go/cmd/evolve/cmdutil/filterenv_promptsloader_test.go:3` — above `import (`

```text
// filterenv_promptsloader_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (see cmd_worktree_test.go's package
// doc in cmd/evolve for the full task/lane background). FilterEvolveEnv and
// NewPromptsLoader had ZERO direct test coverage (0.0% per `go tool
// cover -func`).
```
