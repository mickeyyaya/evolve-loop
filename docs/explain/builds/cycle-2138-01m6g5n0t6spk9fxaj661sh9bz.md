# Build Explanation — Cycle 2138

## Build Binding
- Cycle: 2138
- Base SHA: 2ba8b562b1084663a97bd8ddc2d8a0c718cce348

## Summary
The `gittest` package provides a hermetic credential helper `IsolateHostCredentials()` that isolates host git and GitHub credentials during test execution.

## Rationale
Tests that run `gh` or `git` commands can read host credentials from the user environment.
Uncontrolled host credentials cause test flakes, network prompts, and credential leaks.
One alternative is a per-command `cmd.Env` slice on each command execution.
Per-command configuration requires manual setup in every test and easily misses new tests.
Process-wide isolation in `TestMain` guarantees hermetic credentials across all tests in the package.
The tradeoff is that process-wide environment changes affect all goroutines in the test process.

## Changed Areas
- `go/internal/gittest/credentials.go` — implements `IsolateHostCredentials()` to isolate git and GitHub environment variables.

## Design Decisions
The helper sets `GH_CONFIG_DIR` to a clean temporary directory.
It sets `GIT_CONFIG_GLOBAL` to `os.DevNull`, `GIT_CONFIG_NOSYSTEM` to "1", `GIT_TERMINAL_PROMPT` to "0", and `GIT_SSH_COMMAND` to "ssh -o BatchMode=yes".
It unsets `GH_TOKEN` and `GITHUB_TOKEN` to prevent ambient authentication.
It returns a restore closure that reinstates the original environment and removes the temporary directory.

## Verification
ACS suite predicates `TestC2138_001` through `TestC2138_006` pass with full statement coverage.
Targeted unit tests in `internal/gittest` and `cmd/evolve` verify isolation and restore behavior.

## Compatibility
The helper is additive and does not break existing test helpers.

## Limitations
The helper isolates process environment variables only.
It does not clear credentials stored in the operating system keychain.
