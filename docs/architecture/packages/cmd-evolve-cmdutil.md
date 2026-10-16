# cmd/evolve/cmdutil

## Purpose

`cmd/evolve/cmdutil` holds the small helpers that the `evolve` subcommand handlers share. It is the one home for them: both `package main` and the decomposed `internal/cli/*` command groups import it, so no group copies a helper.

## Design

- **Deliberately minimal.** It is not a CLI framework. Each `cmd_*.go` keeps its own flag parsing. The package holds only the patterns that were copied verbatim across more than ten files.
- **`HasHelp`** is the early-exit gate at the top of a subcommand: It reports if the arguments contain `-h` or `--help`. A single-dash `-help` and `--h` are not help. This matches every existing `cmd_*.go` file.
- **`EnvOrCwd`** returns the root that the environment variable names. It makes the root absolute through `paths.AbsoluteRoot`, so that downstream paths do not depend on the working directory (the cycle 119 fix class). Otherwise it returns the absolute working directory, then `"."`.
- **`ReorderArgs`** moves flag tokens ahead of positional tokens, because `flag.Parse` stops at the first positional. It accepts `probe foo --json` as the bash predicates did. It assumes boolean flags or `--flag=value`, and it does not handle `--flag value`. `ParseInterspersed` handles the value-flag case.
- **`FilterEvolveEnv`** extracts the `EVOLVE_*` and `BRIDGE_*` variables from an environ slice into a map, which is the subset threaded into phase dispatch. Entries without `=` or with an empty key are skipped.
- **`NewPromptsLoader`** roots a `prompts.Loader` at the `EVOLVE_PROMPTS_DIR` override when it is set, else at the project root.

## Invariants

- **A malformed environ entry is skipped**, never a panic and never admitted with a garbage key. Pinned by `TestFilterEvolveEnv`.
- **The override wins, and the project root is ignored once it is set.** An agent that exists only under the project root is not found. Pinned by `TestNewPromptsLoader_HonorsEnvOverride`.
