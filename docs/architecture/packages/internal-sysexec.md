# internal/sysexec

## Purpose

`internal/sysexec` is the one seam for the execution of external commands (git, gh, go, tmux and others). Production code depends on the `RunFunc` type and on `DefaultRunner`. Tests inject an in-memory fake (`go/test/fixtures.FakeExec`) to avoid the fork and exec cost. That cost dominates the wall-clock time of the suite if they do not. A guard test bans raw `os/exec` use outside this seam.

## Design

- **Leaf package.** It imports nothing from `internal/*`, only the standard library. It can then be imported by production code, by white-box tests and by the `go/test/fixtures` harness without an import cycle. The fixtures import core, so the seam cannot live in core.
- **`RunFunc` contract.** Every earlier runner signature reduces to it. It takes a working directory, an environment, piped stdin and streamed stdout and stderr. It gives an exit code and a Go error.
  - A process that runs and exits non-zero returns `(code, nil)`, never an error. Callers branch on the code: `git diff --quiet` with code 1 means "differences" and not "failure". This is load-bearing.
  - The error is non-nil only for unrecoverable failures: binary not found, context cancellation or pipe setup. The exit code is then -1.
  - An empty `dir` inherits the working directory of the caller.
  - A nil `env` inherits `os.Environ()`. A non-nil `env` replaces the process environment, as `os/exec` does.
  - A nil `stdin` means no stdin. A nil `stdout` or `stderr` discards that stream.
- **`DefaultRunner`.** It wraps `exec.CommandContext` and keeps the contract above.
- **Helpers.** `Capture` returns stdout and stderr as strings with the exit code and leaves the failure decision to the caller. `Output` returns trimmed stdout and treats any non-zero exit as an error that carries the code and the stderr. Use it for queries where a non-zero exit is a failure (`git rev-parse`, describe, `symbolic-ref`). A caller that must read a specific non-zero code uses `Capture`.
- **`CombinedOutput`.** It mirrors `exec.Cmd.CombinedOutput`: stdout and stderr interleave into one buffer. Some CLI classifiers frame their reply with header and footer lines on an unspecified stream, so the callers need the merged transcript. An empty stdin is passed as no stdin. A non-zero exit is not folded into the error, because these callers parse the buffer whatever the exit status.
- **Shared writer.** One `*strings.Builder` passed as both stdout and stderr is race-free on the `DefaultRunner` path. `os/exec` detects the shared writer and reuses one pipe and one copy goroutine, as the standard `Cmd.CombinedOutput` does.

## Invariants

- A non-zero exit is `(code, nil)`. Pinned by `sysexec_test.go`.
- The shared-writer path has no data race. Pinned by `TestCombinedOutput_RealRunner_NoRace` under `-race`.
- No package outside `sysexec` and its allowed list runs raw `os/exec`. Pinned by the guard test.
- Only the bridge launch API starts an LLM binary. `TestOnlyTheBridgeStartsAnLLMBinary` (`llm_launch_guard_test.go`) parses the tree and reports each LLM spawn outside it. Row N2 of the isolation matrix runs the same scan. The rules are in [llm-isolation.md](../llm-isolation.md) §5.1.

## Findings

- None open.
