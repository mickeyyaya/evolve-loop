# internal/testhost

## Purpose

`internal/testhost` decides what a test does when a host tool that it needs is absent. On a local host the test skips. In CI on macOS the test fails.

Before 2026-10-10, `.github/workflows/go.yml` ran `make test-integration` on `macos-latest` without tmux. Each real-tmux test skipped with "tmux not installed", and CI was green without the tests. The workflow now installs tmux with `brew`, and this package makes a missing tool a red test in that job.

## Design

- `Tool(t, name)` finds `name` on `PATH` and returns its path. If `name` is missing, it calls `Unavailable` with "`<name>` not installed".
- `Unavailable(t, reason)` skips with `reason`. If `GOOS` is `darwin` and `CI` is `true`, it fails with "CI requires this capability on macOS, but `<reason>`".
- `Host` holds the three inputs: `GOOS`, `Getenv` and `LookPath`. The package functions use the process values. A unit test gives its own `Host`.
- `T` is the part of `testing.TB` that the package uses: `Helper`, `Skip` and `Fatal`.

The callers:

- `internal/bridge`: `requireTmux` and `requireNativeSandbox` (a sandbox that is missing or cannot apply), `TestRealTmuxSandboxTerminalConfinement` (`node`) and the pane-loss replay;
- `internal/adapters/sandbox`: `requireCapableSandbox`;
- `internal/swarm`, `internal/tmuxtest` and `internal/looppreflight`: the real-tmux tests.

## Decisions

- **Only macOS CI fails.** The Linux jobs keep the skip. The SBPL rules and the cycle-1853 incident are macOS only. The Linux job has no `bwrap`, and the real-tmux tests never ran there. To turn them on is a separate change with its own red risk.
- **`CI=true` only.** GitHub Actions sets `CI=true`. Another value, such as `1`, keeps the skip.
- **Test code only.** No production file imports the package.

## Invariants

- A local run without tmux skips; a macOS CI run without tmux fails. Pinned by `TestHost_UnavailableFailsOnlyInMacOSCI` and `TestHost_ToolReturnsThePathOrReportsTheMissingTool`.
- The package functions read the process environment. Pinned by `TestToolAndUnavailable_ReadTheProcessHost`.
- The package is in `go/.cover-strict` at 100 and in `go/.apicover-enforce`.
