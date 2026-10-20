# internal/testhost

## Purpose

`internal/testhost` decides what a test does when a host tool that it needs is absent. On a local host the test skips. In CI on macOS the test fails.

Before 2026-10-10, `.github/workflows/go.yml` ran `make test-integration` on `macos-latest` without tmux. Each real-tmux test skipped with "tmux not installed", and CI was green without the tests. The workflow now installs tmux with `brew`, and this package makes a missing tool a red test in that job.

## Design

- `Tool(t, name)` finds `name` on `PATH` and returns its path. If `name` is missing, it calls `Unavailable` with "`<name>` not installed".
- A CI job can declare a tool required: `TESTHOST_REQUIRED_TOOLS` (`RequiredToolsEnv`) holds a comma-separated list. When a declared tool is missing, `Tool` fails on any host. The three Go test jobs of `.github/workflows/go.yml` (ubuntu Go 1.23, ubuntu Go 1.27, macOS) set `TESTHOST_REQUIRED_TOOLS: tmux`. Thus a runner image without tmux turns the real-tmux tier red, and it does not skip in silence.
- `Unavailable(t, reason)` skips with `reason`. If `GOOS` is `darwin` and `CI` is `true`, it fails with "CI requires this capability on macOS, but `<reason>`".
- `Host` holds the three inputs: `GOOS`, `Getenv` and `LookPath`. The package functions use the process values. A unit test gives its own `Host`.
- `T` is the part of `testing.TB` that the package uses: `Helper`, `Skip` and `Fatal`.

The callers:

- `internal/bridge`: `requireTmux` and `requireNativeSandbox` (a sandbox that is missing or cannot apply), `TestRealTmuxSandboxTerminalConfinement` (`node`) and the pane-loss replay;
- `internal/adapters/sandbox`: `requireCapableSandbox`;
- `internal/swarm`, `internal/tmuxtest` and `internal/looppreflight`: the real-tmux tests.

## Decisions

- **The host rule fails only in macOS CI.** The SBPL rules and the cycle-1853 incident are macOS only, and the Linux jobs have no `bwrap`. The Linux jobs keep the skip for a capability that they do not declare.
- **A declared tool fails on every host.** The ubuntu runner image has tmux. Main run 39422739711 (2026-10-20) passed 22 real-tmux bridge tests in each ubuntu job, with 0 skipped. The ciparitygate backstop says that CI runs that tier. The declaration makes that claim fail loudly when the image drops tmux (independent review of PR #1054).
- **`CI=true` only.** GitHub Actions sets `CI=true`. Another value, such as `1`, keeps the skip.
- **Test code only.** No production file imports the package.

## Invariants

- A local run without tmux skips; a macOS CI run without tmux fails. Pinned by `TestHost_UnavailableFailsOnlyInMacOSCI` and `TestHost_ToolReturnsThePathOrReportsTheMissingTool`.
- A tool that the job declares required fails when it is missing, on any host; an undeclared tool keeps the host rule. Pinned by `TestHost_ToolFailsOnAnyHostWhenTheJobDeclaresTheToolRequired`.
- The package functions read the process environment. Pinned by `TestToolAndUnavailable_ReadTheProcessHost`.
- The package is in `go/.cover-strict` at 100 and in `go/.apicover-enforce`.
