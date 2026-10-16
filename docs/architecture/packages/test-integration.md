# test/integration

## Purpose

`test/integration` holds cross-package tests that wire several `internal/` packages against real temp-directory state without spawning a CLI subagent. They run with `make test-integration`. Tiers: [testing.md](../../../go/docs/testing.md).

## Design

- **Pattern for a new integration test.**
  1. Gate on the external tool. Skip when it is absent, never fail.
  2. `ws := fixtures.NewWorkspace(t).WithGitInit().Build()`.
  3. Shell out to the real tool against `ws.Root` and assert.

## Invariants

- `WithGitInit` must give a work tree that the git binary itself recognises. The git workspace test pins it.
