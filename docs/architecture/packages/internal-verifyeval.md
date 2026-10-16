# internal/verifyeval

## Purpose

`internal/verifyeval` re-executes the commands inside an eval Markdown file against a workspace. It compares the observed outcome to the expectations of the eval. It is the trust-boundary check of the orchestrator before ship: do not trust the claim of the Auditor that the eval passed, run it again. The CLI is `evolve eval verify <eval.md> <workspace>`.

## Design

- **The eval format.** The package reads each fenced `bash` block as one script. The `## Expected` section holds the predicates, one `key: value` line each:

  ```
  exit_code: 0
  stdout_contains: "FAIL"
  stderr_contains: ""
  ```

  The key `stdout_absent` forbids a substring in stdout. The old package doc said that a `!` prefix negates `stdout_contains`, but the code does not read it (a stale fact). Surrounding quotes are stripped. An unknown key is ignored on purpose, for forward compatibility. A malformed `exit_code` is ignored too, so a malformed eval does not crash the verifier. Pinned by `TestParseExpectedLine_AllKeys` and `TestParseExpectedLine_MalformedExitCode`.
- **One fence is one shell.** State and multiline syntax stay inside a fence. Separate fences are isolated executions. A fence that reopens after the `## Expected` section still emits its script (`TestParseEval_BashBlockAfterExpected`).
- **Without a machine-readable `exit_code`, a script must exit 0** when its assertion holds. The package does not interpret the prose around a fence as an expectation. `ExitCode` defaults to zero. The output predicates apply only when present.
- **`CmdRunner` is the seam.** Production runs Bash with pipeline failure propagation (`DefaultRunner`). The script keeps normal Bash status handling, including `$?` to inspect an expected failure. A non-zero exit is not a runner error. Unit tests inject a fake runner, so no real shell runs there. The integration-tagged tests exercise the shell-out path.
- **`go_test_evidence.go` and `shell_commands.go` locate `go test` invocations.** `shellCommands` splits only at unquoted command boundaries. It is a bounded recognizer and not a shell interpreter: Bash stays authoritative when `DefaultRunner` runs the whole script. `shellCommandSubstitutions` returns the executable `$(...)` bodies, including those inside double quotes. A single-quoted substitution is literal text. `shellWords` keeps quoted arguments whole and leaves expansion to Bash.

## Invariants

- **Any failure makes the verdict FAIL, and the scripts after a mismatch still run.** The operator sees the full failure picture. A runner error marks its script failed and the run continues. Pinned by `TestVerify_RunnerError_FAIL`.
- **An empty acceptance definition cannot pass.** An eval with no scripts is an error. Pinned by `TestVerify_NoScripts_Error`.
- **A scanner overflow is an error, never a truncation.** A line longer than `bufio.MaxScanTokenSize` (64 KiB) surfaces `ErrTooLong`. Pinned by `TestParseEval_ScannerError`.
- **A green `go test` run must have executed a test.** `executionEvidenceReason` rejects the successful exit that Go reports when a `-run` selector matches nothing. A recursive command can report the "no tests to run" marker for some packages. One package or test execution is enough.
